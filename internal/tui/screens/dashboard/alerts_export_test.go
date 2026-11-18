package dashboard

import (
	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// RecurrenceAlertTexts exposes recurrenceAlerts to the external test package.
func RecurrenceAlertTexts(recs []app.Recurrence, today, until domain.Date) []string {
	var out []string
	for _, a := range recurrenceAlerts(recs, today, until, domain.LocaleENUS) {
		out = append(out, a.text)
	}
	return out
}
