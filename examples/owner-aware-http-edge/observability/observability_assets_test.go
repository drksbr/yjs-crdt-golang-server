package observability

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v2"
)

type prometheusRuleFile struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
}

type prometheusRuleGroup struct {
	Name  string           `yaml:"name"`
	Rules []prometheusRule `yaml:"rules"`
}

type prometheusRule struct {
	Alert       string            `yaml:"alert"`
	Record      string            `yaml:"record"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func TestObservabilityAssetsAreLoadable(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"grafana-dashboard.json",
		"grafana-oracle-dashboard.json",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) unexpected error: %v", path, err)
			}

			var dashboard map[string]any
			if err := json.Unmarshal(raw, &dashboard); err != nil {
				t.Fatalf("json.Unmarshal(%q) unexpected error: %v", path, err)
			}
			panels, ok := dashboard["panels"].([]any)
			if !ok || len(panels) == 0 {
				t.Fatalf("%q panels = %#v, want non-empty panels array", path, dashboard["panels"])
			}
		})
	}

	for _, path := range []string{
		"prometheus-rules.yml",
		"prometheus-slo-rules.yml",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			rules := readPrometheusRules(t, path)
			if len(rules.Groups) == 0 {
				t.Fatalf("%q has no groups", path)
			}
			for _, group := range rules.Groups {
				if group.Name == "" {
					t.Fatalf("%q has group with empty name", path)
				}
				if len(group.Rules) == 0 {
					t.Fatalf("%q group %q has no rules", path, group.Name)
				}
				for _, rule := range group.Rules {
					if rule.Expr == "" {
						t.Fatalf("%q group %q has rule without expr: %#v", path, group.Name, rule)
					}
					if rule.Alert == "" && rule.Record == "" {
						t.Fatalf("%q group %q has rule without alert or record: %#v", path, group.Name, rule)
					}
					if rule.Alert != "" && rule.Labels["severity"] == "" {
						t.Fatalf("%q alert %q has no severity label", path, rule.Alert)
					}
				}
			}
		})
	}
}

func TestPrometheusSLORulesCoverProductionDimensions(t *testing.T) {
	t.Parallel()

	rules := readPrometheusRules(t, "prometheus-slo-rules.yml")
	records := map[string]bool{}
	alerts := map[string]bool{}
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Record != "" {
				records[rule.Record] = true
				for _, label := range []string{"env", "region", "tenant", "deployment_role"} {
					if !strings.Contains(rule.Expr, label) {
						t.Fatalf("record %q expr does not mention %q: %s", rule.Record, label, rule.Expr)
					}
				}
			}
			if rule.Alert != "" {
				alerts[rule.Alert] = true
				if rule.Labels["slo"] == "" {
					t.Fatalf("alert %q has no slo label", rule.Alert)
				}
			}
		}
	}

	for _, record := range []string{
		"yjsbridge:slo_yhttp_error_ratio_5m",
		"yjsbridge:slo_yhttp_handle_p95_seconds_5m",
		"yjsbridge:slo_owner_lookup_p95_seconds_5m",
		"yjsbridge:slo_remote_owner_handshake_p95_seconds_5m",
		"yjsbridge:slo_authority_loss_rate_5m",
		"yjsbridge:slo_lease_error_ratio_5m",
		"yjsbridge:slo_recovery_tail_lag_max",
	} {
		if !records[record] {
			t.Fatalf("SLO record %q not found", record)
		}
	}

	for _, alert := range []string{
		"YjsBridgeSLOYHTTPAvailabilityBurn",
		"YjsBridgeSLOYHTTPHandleLatencyP95",
		"YjsBridgeSLOOwnerLookupLatencyP95",
		"YjsBridgeSLORemoteOwnerHandshakeLatencyP95",
		"YjsBridgeSLOAuthorityLoss",
		"YjsBridgeSLOLeaseErrorBurn",
		"YjsBridgeSLORecoveryTailLagHigh",
	} {
		if !alerts[alert] {
			t.Fatalf("SLO alert %q not found", alert)
		}
	}
}

func TestPrometheusRulesAvoidCounterFunctionsOnKnownGauges(t *testing.T) {
	t.Parallel()

	knownGauges := []string{
		"yjsbridge_storage_replay_through_offset",
		"yjsbridge_storage_replay_last_epoch",
		"yjsbridge_storage_recovery_checkpoint_offset",
		"yjsbridge_storage_recovery_last_offset",
		"yjsbridge_storage_recovery_tail_lag_updates",
		"yjsbridge_storage_recovery_last_epoch",
		"yjsbridge_storage_compaction_through_offset",
		"yjsbridge_storage_compaction_last_epoch",
	}

	for _, path := range []string{"prometheus-rules.yml", "prometheus-slo-rules.yml"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			rules := readPrometheusRules(t, path)
			for _, group := range rules.Groups {
				for _, rule := range group.Rules {
					for _, gauge := range knownGauges {
						if strings.Contains(rule.Expr, "rate("+gauge+"[") || strings.Contains(rule.Expr, "increase("+gauge+"[") {
							t.Fatalf("%q rule %q uses counter function on gauge %q: %s", path, ruleName(rule), gauge, rule.Expr)
						}
					}
				}
			}
		})
	}
}

func TestPrometheusRulesUseAnchoredNegativeRegexes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"prometheus-rules.yml", "prometheus-slo-rules.yml"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			rules := readPrometheusRules(t, path)
			for _, group := range rules.Groups {
				for _, rule := range group.Rules {
					if !strings.Contains(rule.Expr, "!~") {
						continue
					}
					for _, fragment := range strings.Split(rule.Expr, "!~")[1:] {
						fragment = strings.TrimSpace(fragment)
						if !strings.HasPrefix(fragment, `"^`) {
							t.Fatalf("%q rule %q has unanchored negative regex in expr: %s", path, ruleName(rule), rule.Expr)
						}
					}
				}
			}
		})
	}
}

func readPrometheusRules(t *testing.T, path string) prometheusRuleFile {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) unexpected error: %v", path, err)
	}

	var rules prometheusRuleFile
	if err := yaml.Unmarshal(raw, &rules); err != nil {
		t.Fatalf("yaml.Unmarshal(%q) unexpected error: %v", path, err)
	}
	return rules
}

func ruleName(rule prometheusRule) string {
	if rule.Alert != "" {
		return rule.Alert
	}
	return rule.Record
}
