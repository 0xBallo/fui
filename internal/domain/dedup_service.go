package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// DedupClass is the outcome of deduplicating one incoming transaction.
type DedupClass string

// Dedup classes.
const (
	DedupNew       DedupClass = "new"
	DedupDuplicate DedupClass = "duplicate"
	DedupDoubtful  DedupClass = "doubtful"
)

// DedupResult is the classification of one transaction against existing ones.
type DedupResult struct {
	Class    DedupClass
	Matched  []TransactionID
	Incoming string
}

// NormalizeDescription trims, case-folds and collapses internal whitespace.
func NormalizeDescription(description string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(description)), " "))
}

// Fingerprint computes sha256(account|date|amount|normalized description).
func Fingerprint(accountID AccountID, date Date, amount Money, description string) (string, error) {
	if accountID == 0 {
		return "", missingField("account_id")
	}
	if date.IsZero() {
		return "", missingField("date")
	}
	if amount.Currency().IsZero() {
		return "", missingField("amount")
	}
	input := fmt.Sprintf("%d|%s|%d|%s", accountID, date, amount.Minor(), NormalizeDescription(description))
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:]), nil
}

// ClassifyDedup classifies incoming as new/duplicate/doubtful against existing.
func ClassifyDedup(incoming Transaction, existing []Transaction) (DedupResult, error) {
	if incoming.AccountID == 0 {
		return DedupResult{}, missingField("account_id")
	}
	if incoming.Date.IsZero() {
		return DedupResult{}, missingField("date")
	}
	if incoming.Amount.Currency().IsZero() {
		return DedupResult{}, missingField("amount")
	}
	match := matchByExternalID(strings.TrimSpace(incoming.ExternalID), existing)
	if len(match) > 0 {
		return DedupResult{Class: DedupDuplicate, Matched: match, Incoming: incoming.ExternalID}, nil
	}
	incomingFP, err := txFingerprint(incoming)
	if err != nil {
		return DedupResult{}, err
	}
	match, err = matchByFingerprint(incomingFP, existing)
	if err != nil {
		return DedupResult{}, err
	}
	if len(match) > 0 {
		return DedupResult{Class: DedupDuplicate, Matched: match, Incoming: incomingFP}, nil
	}
	match, err = matchDoubtful(incoming, existing)
	if err != nil {
		return DedupResult{}, err
	}
	if len(match) > 0 {
		return DedupResult{Class: DedupDoubtful, Matched: match}, nil
	}
	return DedupResult{Class: DedupNew}, nil
}

func matchByExternalID(externalID string, existing []Transaction) []TransactionID {
	if externalID == "" {
		return nil
	}
	var matched []TransactionID
	for _, tx := range existing {
		if strings.TrimSpace(tx.ExternalID) == externalID {
			matched = append(matched, tx.ID)
		}
	}
	return matched
}

func matchByFingerprint(incomingFP string, existing []Transaction) ([]TransactionID, error) {
	var matched []TransactionID
	for _, tx := range existing {
		fp, err := txFingerprint(tx)
		if err != nil {
			return nil, err
		}
		if fp == incomingFP {
			matched = append(matched, tx.ID)
		}
	}
	return matched, nil
}

func txFingerprint(tx Transaction) (string, error) {
	if tx.Fingerprint != "" {
		return strings.TrimSpace(strings.ToLower(tx.Fingerprint)), nil
	}
	return Fingerprint(tx.AccountID, tx.Date, tx.Amount, tx.Description)
}

func matchDoubtful(incoming Transaction, existing []Transaction) ([]TransactionID, error) {
	var matched []TransactionID
	for _, tx := range existing {
		if tx.AccountID != incoming.AccountID {
			continue
		}
		cmp, err := tx.Amount.Cmp(incoming.Amount)
		if err != nil {
			return nil, err
		}
		if cmp != 0 {
			continue
		}
		if dateDistanceDays(tx.Date, incoming.Date) > 2 {
			continue
		}
		matched = append(matched, tx.ID)
	}
	return matched, nil
}

func dateDistanceDays(a, b Date) int {
	delta := b.In(time.UTC).Sub(a.In(time.UTC))
	if delta < 0 {
		delta = -delta
	}
	return int(delta / (24 * time.Hour))
}
