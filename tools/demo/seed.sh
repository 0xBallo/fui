#!/usr/bin/env bash
# seed.sh builds fui and fills a throwaway archive with 12 months of synthetic data
# for manual testing. Everything lives under DIR (default .scratch/demo): HOME and the
# XDG dirs are redirected there, so the real archive is never touched.
#
#   tools/demo/seed.sh [DIR]
set -euo pipefail

DIR=$(mkdir -p "${1:-.scratch/demo}" && cd "${1:-.scratch/demo}" && pwd)
if [ -e "$DIR/data/fui/fui.db" ]; then
	echo "archive already exists in $DIR; remove the directory to reseed" >&2
	exit 1
fi
go build -o "$DIR/fui" ./cmd/fui # before the HOME override, so the real module cache is used
export HOME="$DIR" XDG_CONFIG_HOME="$DIR/config" XDG_DATA_HOME="$DIR/data" XDG_STATE_HOME="$DIR/state"
unset FUI_CONFIG FUI_DB

# Global flags go last: add and transfer parse their own flags and need the amount first.
fui() { "$DIR/fui" "$@" --no-input --yes >/dev/null; }

RANDOM=42 # deterministic amounts
# money prints a negative amount between $1 and $2 euros in it-IT format, e.g. -42,17.
money() { local c=$(($1 * 100 + RANDOM % (($2 - $1) * 100 + 1))); printf -- '-%d,%02d' $((c / 100)) $((c % 100)); }
pick() { local a=("$@"); echo "${a[RANDOM % $#]}"; }

year=$(date +%Y) month=$((10#$(date +%m))) today=$((10#$(date +%d)))
# ym N prints YYYY-MM for N months before the current month.
ym() { local m=$((year * 12 + month - 1 - $1)); printf '%04d-%02d' $((m / 12)) $((m % 12 + 1)); }
start="$(ym 12)-01"

fui init --currency EUR --preset full
fui account add Checking --kind checking --currency EUR --opening-balance 3000 --opened-on "$start"
fui account add Savings --kind savings --currency EUR --opening-balance 10000 --opened-on "$start"
fui account add "Credit Card" --kind credit_card --currency EUR --opened-on "$start"
fui account add Cash --kind cash --currency EUR --opening-balance 200 --opened-on "$start"
fui account add "US Account" --kind checking --currency USD --opening-balance 1500 --opened-on "$start"
fui account add Broker --kind investment --currency EUR --opened-on "$start"
fui fx set USD 1,08 --date "$start"

for back in $(seq 11 -1 0); do
	m=$(ym "$back")
	# tx DAY ACCOUNT CATEGORY AMOUNT DESCRIPTION; skips days still in the future.
	tx() {
		[ "$back" -eq 0 ] && [ "$1" -gt "$today" ] && return
		fui add "$4" "$5" -a "$2" -c "$3" --date "$m-$(printf %02d "$1")"
	}
	tx 1 Checking Housing -850 "Affitto"
	tx 10 Checking Utilities "$(money 55 120)" "Enel Energia"
	tx 12 Checking Utilities "$(money 25 45)" "Fastweb"
	tx 15 "Credit Card" Subscriptions -12,99 "Netflix"
	tx 20 "Credit Card" Subscriptions -10,99 "Spotify"
	tx 27 Checking Salary 2400 "Stipendio ACME Srl"
	for _ in $(seq 7); do tx $((RANDOM % 28 + 1)) Checking Groceries "$(money 15 110)" "$(pick Esselunga Coop Conad Lidl)"; done
	for _ in $(seq 4); do tx $((RANDOM % 28 + 1)) "Credit Card" Dining "$(money 12 70)" "$(pick 'Pizzeria Da Mario' 'Sushi Ko' 'Bar Centrale')"; done
	for _ in $(seq 4); do tx $((RANDOM % 28 + 1)) Cash Transport "$(money 2 15)" "$(pick ATM Taxi Parcheggio)"; done
	tx $((RANDOM % 28 + 1)) Checking Transport "$(money 50 80)" "Eni Carburante"
	tx $((RANDOM % 28 + 1)) "Credit Card" Shopping "$(money 20 150)" "$(pick Amazon Zalando IKEA)"
	tx $((RANDOM % 28 + 1)) "US Account" Entertainment "$(money 10 40)" "Steam"
	[ $((back % 3)) -eq 0 ] && tx 18 Checking Freelance 600 "Fattura consulenza"
	[ "$back" -eq 0 ] || fui transfer 300 --from Checking --to Savings --date "$m-28"
	[ "$back" -eq 0 ] || fui transfer 250 --from Checking --to "Credit Card" --date "$m-25"
	[ "$back" -eq 0 ] || fui transfer 50 --from Checking --to Cash --date "$m-05"
done
# One outlier so insights has an anomaly to report.
fui add -1299 "MediaWorld TV OLED" -a "Credit Card" -c Shopping --date "$(ym 1)-14"

for b in "Groceries 400" "Dining 150" "Transport 150" "Shopping 200" "Subscriptions 30"; do fui budget set $b; done

next="$(ym -1)"
fui recurring add --account Checking --amount -850 --description Affitto --category Housing --every 1 --unit months --day-of-month 1 --start "$next-01"
fui recurring add --account Checking --amount 2400 --description "Stipendio ACME Srl" --category Salary --every 1 --unit months --day-of-month 27 --start "$next-27"

fui loan add "Prestito Auto" --kind loan --principal 15000 --term-months 60 --rate-type fixed --rate 5,5 --debit Checking --disbursed "$(ym 6)-05"
fui loan run
fui asset add Auto --kind vehicle --value 18000 --purchase-date "$(ym 6)-05" --loan "Prestito Auto"
fui asset value Auto 16500 --date "$(ym 0)-01"

fui security add "Vanguard FTSE All-World" --kind etf --ticker VWCE --currency EUR
fui security add Bitcoin --kind crypto --ticker BTC --currency EUR
for back in 10 7 4 1; do
	fui invest buy VWCE --account Broker --quantity 5 --price $((110 + (10 - back) * 2)) --fees 2 --cash-account Checking --date "$(ym "$back")-05"
done
fui invest buy BTC --account Broker --quantity 0.02 --price 60000 --cash-account Checking --date "$(ym 8)-12"
fui security price VWCE 131,40 --date "$(ym 0)-01"
fui security price BTC 82000 --date "$(ym 0)-01"

cat <<EOF
Demo archive ready in $DIR. Launch it with:
  HOME=$DIR XDG_CONFIG_HOME=$DIR/config XDG_DATA_HOME=$DIR/data XDG_STATE_HOME=$DIR/state $DIR/fui
EOF
