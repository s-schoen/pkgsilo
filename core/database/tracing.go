package database

import "go.opentelemetry.io/otel/attribute"

const (
	// tracerName is the instrumentation scope of the transaction spans, named like the meter.
	tracerName = meterName
	// transactionSpanName is the span of a transaction that InTx begins.
	transactionSpanName = "pkgsilo.database.transaction"
	// outcomeKey tells whether the transaction committed or rolled back: outcomeCommit or outcomeRollback. A
	// span whose BEGIN failed has none, since no transaction began.
	outcomeKey      = attribute.Key("pkgsilo.database.transaction.outcome")
	outcomeCommit   = "commit"
	outcomeRollback = "rollback"
)
