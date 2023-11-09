package precheck

import "github.com/prometheus/client_golang/prometheus"

var (
	basicCheckDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "axiom_ledger",
			Subsystem: "pre_check",
			Name:      "basic_check_duration_seconds",
			Help:      "The average latency of basic check",
		},
		[]string{"type"},
	)
	verifySignatureDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "axiom_ledger",
			Subsystem: "pre_check",
			Name:      "verify_signature_duration_seconds",
			Help:      "The average latency of verify signature",
		},
		[]string{"type"},
	)
	verifyBlanceDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "axiom_ledger",
			Subsystem: "pre_check",
			Name:      "verify_balance_duration_seconds",
			Help:      "The average latency of verify balance",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(basicCheckDuration)
	prometheus.MustRegister(verifySignatureDuration)
	prometheus.MustRegister(verifyBlanceDuration)
}
