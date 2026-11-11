// Package importing holds the import/export use cases: the import pipeline (plan without
// writes, apply in one transaction), undo of an import batch, export of transactions in CSV,
// JSON and fui/1 format, the full dump and the connector use cases (list, show, test).
//
// It depends on ports only: Source yields fui/1 records and Connectors finds and opens
// connectors. internal/connector implements both; cmd/fui wires them.
package importing
