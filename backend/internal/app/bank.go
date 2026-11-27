package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/RostKostia/SKA/backend/internal/boc"
)

var bankPermissions = boc.Permissions{Accounts: boc.AccountPermissions{TransactionHistory: true, Balance: true, Details: true}}

func (a *App) requireBoC() error {
	if a.BoC == nil {
		return conflict("bank_mock", "KERDOS_BANK is mock, so the sample account is used. Set KERDOS_BANK=boc and the BOC_* settings to connect the BoC sandbox")
	}
	return nil
}

// BankConnect starts BoC consent and returns the 1Bank page to send the owner's browser to.
func (a *App) BankConnect(ctx context.Context, u User) (string, error) {
	if err := a.requireBoC(); err != nil {
		return "", err
	}
	sub, err := a.BoC.CreateSubscription(ctx, bankPermissions)
	if err != nil {
		return "", err
	}
	if _, err := a.DB.ExecContext(ctx, `INSERT INTO bank_connections (provider, subscription_id, status, permissions, created_by)
		VALUES ('boc', $1, 'pending', $2::jsonb, $3)`, sub.SubscriptionID, jsonArg(bankPermissions), nullable(u.ID)); err != nil {
		return "", err
	}
	return a.BoC.AuthorizeURL(sub.SubscriptionID), nil
}

// BankCallback finishes consent with the code from the 1Bank redirect, stores the approved
// accounts (the first current account becomes the primary one) and syncs them.
func (a *App) BankCallback(ctx context.Context, code string) error {
	if err := a.requireBoC(); err != nil {
		return err
	}
	var connID, subID string
	err := a.DB.QueryRowContext(ctx, `SELECT id::text, subscription_id FROM bank_connections
		WHERE provider = 'boc' AND status = 'pending' ORDER BY created_at DESC LIMIT 1`).Scan(&connID, &subID)
	if errors.Is(err, sql.ErrNoRows) {
		return conflict("no_pending_connection", "no bank connection is waiting for consent")
	}
	if err != nil {
		return err
	}
	userToken, err := a.BoC.ExchangeCode(ctx, code)
	if err != nil {
		return err
	}
	s, err := a.BoC.Activate(ctx, userToken, subID)
	if err != nil {
		return err
	}
	if s.Status != "ACTV" {
		return conflict("not_active", "BoC did not activate the subscription (status "+s.Status+")")
	}
	accounts, err := a.BoC.Accounts(ctx, subID)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, sa := range s.SelectedAccounts {
		selected[sa.AccountID] = true
	}
	err = a.tx(ctx, func(tx *sql.Tx) error {
		var starts, expires any
		if s.StartsOn != nil {
			starts = s.StartsOn.Format("2006-01-02")
		}
		if s.ExpiresOn != nil {
			expires = s.ExpiresOn.Format("2006-01-02")
		}
		perms, _ := json.Marshal(boc.Permissions{Accounts: s.Accounts, Payments: s.Payments})
		if _, err := tx.ExecContext(ctx, `UPDATE bank_connections SET status = 'active', starts_on = $2::date, expires_on = $3::date, permissions = $4::jsonb
			WHERE id = $1`, connID, starts, expires, string(perms)); err != nil {
			return err
		}
		primary := ""
		for _, acc := range accounts {
			if len(selected) > 0 && !selected[acc.AccountID] {
				continue
			}
			name := acc.Name
			if acc.Alias != "" {
				name = acc.Alias
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO bank_accounts (bank_connection_id, boc_account_id, bank_id, iban, account_type, name, currency)
				VALUES ($1, $2, $3, $4, $5, $6, coalesce(nullif($7, ''), 'EUR'))
				ON CONFLICT (boc_account_id) DO UPDATE SET bank_connection_id = EXCLUDED.bank_connection_id, iban = EXCLUDED.iban,
					account_type = EXCLUDED.account_type, name = EXCLUDED.name`,
				connID, acc.AccountID, acc.BankID, acc.IBAN, acc.Type, name, acc.Currency); err != nil {
				return err
			}
			if primary == "" && acc.Type == "CURRENT" {
				primary = acc.AccountID
			}
		}
		if primary != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE bank_accounts SET is_primary = false WHERE is_primary`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE bank_accounts SET is_primary = true WHERE boc_account_id = $1`, primary); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = a.BankSync(ctx)
	return err
}

type SyncResult struct {
	Accounts        int    `json:"accounts"`
	NewTransactions int    `json:"new_transactions"`
	MatchedPayments int    `json:"matched_payments"`
	Note            string `json:"note,omitempty"`
}

var invoiceNumberRe = regexp.MustCompile(`[A-Z]{2,}-\d{4}-\d{4}`)

// BankSync fetches the balance and the last 90 days of transactions of every connected BoC
// account. A credit whose description holds an open invoice's number becomes its payment.
func (a *App) BankSync(ctx context.Context) (SyncResult, error) {
	if a.BoC == nil {
		return SyncResult{Note: "KERDOS_BANK is mock: nothing to fetch"}, nil
	}
	var res SyncResult
	rows, err := a.DB.QueryContext(ctx, `SELECT ba.id::text, ba.boc_account_id, bc.subscription_id, bc.id::text
		FROM bank_accounts ba JOIN bank_connections bc ON bc.id = ba.bank_connection_id
		WHERE bc.provider = 'boc' AND bc.status = 'active'`)
	if err != nil {
		return res, err
	}
	type acct struct{ id, bocID, sub, conn string }
	var list []acct
	for rows.Next() {
		var x acct
		if err := rows.Scan(&x.id, &x.bocID, &x.sub, &x.conn); err != nil {
			rows.Close()
			return res, err
		}
		list = append(list, x)
	}
	rows.Close()
	now := time.Now()
	for _, x := range list {
		bal, err := a.BoC.Balance(ctx, x.sub, x.bocID)
		if err != nil {
			return res, err
		}
		current := *bal.AvailableCt
		if bal.CurrentCt != nil {
			current = *bal.CurrentCt
		}
		if _, err := a.DB.ExecContext(ctx, `INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source) VALUES ($1, $2, $3, 'boc')`,
			x.id, *bal.AvailableCt, current); err != nil {
			return res, err
		}
		txs, err := a.BoC.Statement(ctx, x.sub, x.bocID, now.AddDate(0, 0, -90), now, 500)
		if err != nil {
			return res, err
		}
		for _, t := range txs {
			var newID string
			var value any
			if t.ValueDate != nil {
				value = t.ValueDate.Format("2006-01-02")
			}
			err := a.DB.QueryRowContext(ctx, `INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, value_date, dc_ind,
					amount_cents, currency, transaction_type, description, raw)
				VALUES ($1, $2, $3::date, $4::date, $5, $6, $7, $8, $9, $10::jsonb)
				ON CONFLICT (boc_transaction_id) DO NOTHING RETURNING id::text`,
				x.id, t.ID, t.PostingDate.Format("2006-01-02"), value, nullable(t.DCInd), t.AmountCents, t.Currency,
				nullable(t.Type), nullable(t.Description), string(t.Raw)).Scan(&newID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return res, err
			}
			res.NewTransactions++
			if t.DCInd == "credit" {
				if ok, err := a.matchPayment(ctx, newID, t); err != nil {
					return res, err
				} else if ok {
					res.MatchedPayments++
				}
			}
		}
		res.Accounts++
		if _, err := a.DB.ExecContext(ctx, `UPDATE bank_connections SET last_synced_at = now() WHERE id = $1`, x.conn); err != nil {
			return res, err
		}
	}
	a.Kick()
	return res, nil
}

func (a *App) matchPayment(ctx context.Context, txID string, t boc.Transaction) (bool, error) {
	number := invoiceNumberRe.FindString(t.Description)
	if number == "" {
		return false, nil
	}
	var invID string
	var outstanding int64
	err := a.DB.QueryRowContext(ctx, `SELECT id::text, outstanding_cents FROM invoice_overview
		WHERE number = $1 AND direction = 'receivable' AND status = 'sent' AND outstanding_cents > 0`, number).Scan(&invID, &outstanding)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	amount := min(t.AmountCents, outstanding)
	_, err = a.DB.ExecContext(ctx, `INSERT INTO payments (invoice_id, amount_cents, received_on, method, bank_transaction_id, provider, external_id)
		VALUES ($1, $2, $3::date, 'bank_transfer', $4, 'boc', $5)`, invID, amount, t.PostingDate.Format("2006-01-02"), txID, t.ID)
	return err == nil, err
}
