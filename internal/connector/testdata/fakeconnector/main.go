// Command fakeconnector is a configurable fui connector used to test the runner. The behaviour
// of fetch comes from config.mode on stdin; describe reads FAKE_MODE from the environment.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	switch os.Args[1] {
	case "describe":
		describe()
	case "fetch":
		fetch()
	default:
		os.Exit(1)
	}
}

func describe() {
	switch os.Getenv("FAKE_MODE") {
	case "fail":
		fmt.Fprintln(os.Stderr, "describe failed")
		os.Exit(1)
	case "hang":
		time.Sleep(time.Hour)
	case "badproto":
		fmt.Println(`{"protocol":"fui/2","name":"fake","version":"1"}`)
		return
	case "garbage":
		fmt.Println("not json")
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(fuiproto.DescribeResponse{Protocol: fuiproto.ProtocolFUI1, Name: "fake", Version: "1.2.3"})
}

func fetch() {
	var req fuiproto.FetchRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, "bad request:", err)
		os.Exit(fuiproto.ExitCodeBadConfig)
	}
	mode, _ := req.Config["mode"].(string)
	enc := fuiproto.NewJSONLEncoder(os.Stdout)
	emit := func(r fuiproto.Record) {
		if err := enc.Encode(r); err != nil {
			os.Exit(1)
		}
	}
	meta := fuiproto.Record{Type: fuiproto.RecordTypeMeta, Meta: &fuiproto.MetaRecord{
		Protocol: fuiproto.ProtocolFUI1, Connector: "fake", Version: "1", Source: fuiproto.SourceMeta{File: "fake.dat", SHA256: "fake"},
	}}
	acct := fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &fuiproto.AccountRecord{
		Ref: "fake", Name: "Fake", Currency: "EUR", Kind: fuiproto.AccountKindChecking,
	}}
	tx := func(id, desc string) fuiproto.Record {
		return fuiproto.Record{Type: fuiproto.RecordTypeTransaction, Transaction: &fuiproto.TransactionRecord{
			Account: "fake", ID: id, Date: "2026-10-01", Amount: -100, Currency: "EUR", Description: desc,
			Status: fuiproto.TransactionStatusBooked,
		}}
	}
	end := func(n int) fuiproto.Record {
		return fuiproto.Record{Type: fuiproto.RecordTypeEnd, End: &fuiproto.EndRecord{Transactions: n}}
	}
	switch mode {
	case "exit3":
		fmt.Fprintln(os.Stderr, "cannot read the source")
		os.Exit(fuiproto.ExitCodeUnreadableSource)
	case "garbage":
		fmt.Println("this is not json")
	case "bigline":
		emit(meta)
		fmt.Println(strings.Repeat("x", 2<<20))
	case "hang":
		emit(meta)
		time.Sleep(time.Hour)
	case "truncated":
		emit(meta)
		emit(acct)
		emit(tx("t1", "never closed"))
	case "fail-after":
		emit(meta)
		emit(acct)
		emit(tx("t1", "complete stream"))
		emit(end(1))
		fmt.Fprintln(os.Stderr, "but I crashed")
		os.Exit(1)
	case "env":
		var fui []string
		for _, kv := range os.Environ() {
			if strings.HasPrefix(kv, "FUI_") {
				fui = append(fui, kv)
			}
		}
		sort.Strings(fui)
		wd, _ := os.Getwd()
		emit(meta)
		emit(acct)
		emit(tx("env", "fui vars: "+strings.Join(append([]string{"-"}, fui...), " ")))
		emit(tx("cwd", "cwd: "+filepath.Base(wd)))
		emit(tx("extra", "extra: "+os.Getenv("FAKE_EXTRA")))
		emit(end(3))
	case "stdin":
		file := "<nil>"
		if req.File != nil {
			file = *req.File
		}
		since := "<nil>"
		if req.Since != nil {
			since = *req.Since
		}
		emit(meta)
		emit(acct)
		emit(tx("stdin", fmt.Sprintf("file=%s since=%s echo=%v", file, since, req.Config["echo"])))
		emit(end(1))
	default: // "ok", "stderr", "noisy"
		if mode == "stderr" {
			fmt.Fprintln(os.Stderr, "hello from stderr")
		}
		if mode == "noisy" {
			fmt.Fprint(os.Stderr, strings.Repeat("e", 200<<10))
		}
		emit(meta)
		emit(acct)
		emit(tx("t1", "first"))
		emit(tx("t2", "second"))
		emit(end(2))
	}
}
