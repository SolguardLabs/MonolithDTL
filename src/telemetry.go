package main

import (
	"sort"
	"strings"
)

type MetricPoint struct {
	Name   string            `json:"name"`
	Value Amount            `json:"value"`
	Tags   map[string]string `json:"tags"`
}

type TelemetrySnapshot struct {
	Clock   int64         `json:"clock"`
	Metrics []MetricPoint `json:"metrics"`
}

type EventFilter struct {
	Product  ProductKind
	Kind     string
	Account  string
	Severity string
}

func FilterEvents(events []Event, filter EventFilter) []Event {
	out := make([]Event, 0)
	for _, event := range events {
		if filter.Product != "" && event.Product != filter.Product {
			continue
		}
		if filter.Kind != "" && event.Kind != filter.Kind {
			continue
		}
		if filter.Account != "" && event.Account != filter.Account {
			continue
		}
		if filter.Severity != "" && event.Severity != filter.Severity {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i int, j int) bool {
		if out[i].Clock == out[j].Clock {
			return out[i].ID < out[j].ID
		}
		return out[i].Clock < out[j].Clock
	})
	return out
}

func CountEvents(events []Event, kind string) int {
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

func BuildTelemetry(report ScenarioReport) TelemetrySnapshot {
	metrics := []MetricPoint{
		metric("risk.local_debt", report.Risk.LocalDebt, "scenario", report.Scenario),
		metric("risk.global_debt", report.Risk.GlobalDebt, "scenario", report.Scenario),
		metric("risk.debt_drift", report.Risk.DebtDrift, "scenario", report.Scenario),
		metric("risk.reserve_leak", report.Risk.ReserveLeak, "scenario", report.Scenario),
		metric("events.total", Amount(len(report.Events)), "scenario", report.Scenario),
		metric("redemptions.processed", Amount(report.Risk.ProcessedRedemptions), "scenario", report.Scenario),
		metric("redemptions.rejected", Amount(report.Risk.RejectedRedemptions), "scenario", report.Scenario),
	}
	for _, line := range report.Reconciliation {
		metrics = append(metrics, MetricPoint{
			Name:  "reconcile.drift",
			Value: line.Drift,
			Tags: map[string]string{
				"scenario": report.Scenario,
				"account":  line.Account,
				"asset":    line.Asset,
				"severity": line.Severity,
			},
		})
	}
	sort.Slice(metrics, func(i int, j int) bool {
		left := metrics[i].Name + strings.Join(tagPairs(metrics[i].Tags), ",")
		right := metrics[j].Name + strings.Join(tagPairs(metrics[j].Tags), ",")
		return left < right
	})
	return TelemetrySnapshot{Clock: report.Clock, Metrics: metrics}
}

func metric(name string, value Amount, tagKey string, tagValue string) MetricPoint {
	return MetricPoint{
		Name:  name,
		Value: value,
		Tags: map[string]string{tagKey: tagValue},
	}
}

func tagPairs(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+tags[key])
	}
	return out
}

func TelemetryDigest(snapshot TelemetrySnapshot) string {
	parts := []string{AmountToString(Amount(snapshot.Clock))}
	for _, metric := range snapshot.Metrics {
		parts = append(parts, metric.Name+":"+AmountToString(metric.Value)+":"+strings.Join(tagPairs(metric.Tags), ","))
	}
	sort.Strings(parts)
	return Digest(parts...)
}

func SeverityCounts(events []Event) map[string]int {
	out := make(map[string]int)
	for _, event := range events {
		out[event.Severity]++
	}
	return out
}

func ProductEventCounts(events []Event) map[string]int {
	out := make(map[string]int)
	for _, event := range events {
		out[string(event.Product)]++
	}
	return out
}

func HasCriticalEvent(events []Event) bool {
	for _, event := range events {
		if event.Severity == "critical" {
			return true
		}
	}
	return false
}

