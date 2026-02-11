package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ActiveCallsProvider exposes the number of active calls.
type ActiveCallsProvider interface {
	GetActiveCallCount() int
}

// RegistrationCounter returns the number of active SIP registrations.
type RegistrationCounter interface {
	Count(ctx context.Context) (int64, error)
}

// TrunkStatusEntry represents the status of a single trunk for metrics.
type TrunkStatusEntry struct {
	TrunkID int64
	Name    string
	Status  string
}

// TrunkStatusProvider exposes trunk registration statuses.
type TrunkStatusProvider interface {
	GetAllTrunkStatuses() []TrunkStatusEntry
}

// CDRDirectionCounter returns CDR counts grouped by direction.
type CDRDirectionCounter interface {
	CountByDirection(ctx context.Context) (map[string]int64, error)
}

// RTPStatsProvider returns aggregate RTP statistics.
type RTPStatsProvider interface {
	ActiveSessionCount() int
	AggregatePacketsForwarded() uint64
	AggregatePacketsDropped() uint64
	AggregateBytesForwarded() uint64
}

// VoicemailCounter returns the total voicemail message count across all boxes.
type VoicemailCounter interface {
	CountAll(ctx context.Context) (int64, error)
}

// Collector is a prometheus.Collector that gathers FlowPBX metrics at scrape time.
type Collector struct {
	activeCalls   ActiveCallsProvider
	registrations RegistrationCounter
	trunks        TrunkStatusProvider
	cdrs          CDRDirectionCounter
	rtp           RTPStatsProvider
	voicemail     VoicemailCounter
	startTime     time.Time

	// Metric descriptors.
	activeCallsDesc       *prometheus.Desc
	registrationsDesc     *prometheus.Desc
	trunkStatusDesc       *prometheus.Desc
	callsTotalDesc        *prometheus.Desc
	rtpSessionsDesc       *prometheus.Desc
	rtpPacketsDesc        *prometheus.Desc
	rtpPacketsDroppedDesc *prometheus.Desc
	rtpBytesDesc          *prometheus.Desc
	voicemailMessagesDesc *prometheus.Desc
	uptimeDesc            *prometheus.Desc
}

// NewCollector creates a new metrics collector. Any provider may be nil if unavailable.
func NewCollector(
	activeCalls ActiveCallsProvider,
	registrations RegistrationCounter,
	trunks TrunkStatusProvider,
	cdrs CDRDirectionCounter,
	rtp RTPStatsProvider,
	voicemail VoicemailCounter,
	startTime time.Time,
) *Collector {
	return &Collector{
		activeCalls:   activeCalls,
		registrations: registrations,
		trunks:        trunks,
		cdrs:          cdrs,
		rtp:           rtp,
		voicemail:     voicemail,
		startTime:     startTime,

		activeCallsDesc: prometheus.NewDesc(
			"flowpbx_active_calls",
			"Number of currently active calls (ringing + answered)",
			nil, nil,
		),
		registrationsDesc: prometheus.NewDesc(
			"flowpbx_registered_devices",
			"Number of currently registered SIP devices",
			nil, nil,
		),
		trunkStatusDesc: prometheus.NewDesc(
			"flowpbx_trunk_status",
			"Trunk registration status (1=registered, 0=other)",
			[]string{"trunk_id", "name", "status"}, nil,
		),
		callsTotalDesc: prometheus.NewDesc(
			"flowpbx_calls_total",
			"Total number of calls processed (from CDR)",
			[]string{"direction"}, nil,
		),
		rtpSessionsDesc: prometheus.NewDesc(
			"flowpbx_rtp_sessions_active",
			"Number of active RTP media sessions",
			nil, nil,
		),
		rtpPacketsDesc: prometheus.NewDesc(
			"flowpbx_rtp_packets_forwarded_total",
			"Total RTP packets forwarded across all active sessions",
			nil, nil,
		),
		rtpPacketsDroppedDesc: prometheus.NewDesc(
			"flowpbx_rtp_packets_dropped_total",
			"Total RTP packets dropped across all active sessions",
			nil, nil,
		),
		rtpBytesDesc: prometheus.NewDesc(
			"flowpbx_rtp_bytes_forwarded_total",
			"Total RTP bytes forwarded across all active sessions",
			nil, nil,
		),
		voicemailMessagesDesc: prometheus.NewDesc(
			"flowpbx_voicemail_messages",
			"Total voicemail messages across all mailboxes",
			nil, nil,
		),
		uptimeDesc: prometheus.NewDesc(
			"flowpbx_uptime_seconds",
			"Seconds since the FlowPBX process started",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.activeCallsDesc
	ch <- c.registrationsDesc
	ch <- c.trunkStatusDesc
	ch <- c.callsTotalDesc
	ch <- c.rtpSessionsDesc
	ch <- c.rtpPacketsDesc
	ch <- c.rtpPacketsDroppedDesc
	ch <- c.rtpBytesDesc
	ch <- c.voicemailMessagesDesc
	ch <- c.uptimeDesc
}

// Collect implements prometheus.Collector. It queries all providers at scrape time.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Active calls gauge.
	if c.activeCalls != nil {
		ch <- prometheus.MustNewConstMetric(
			c.activeCallsDesc, prometheus.GaugeValue,
			float64(c.activeCalls.GetActiveCallCount()),
		)
	}

	// Registered devices gauge.
	if c.registrations != nil {
		count, err := c.registrations.Count(ctx)
		if err != nil {
			slog.Error("metrics: failed to count registrations", "error", err)
		} else {
			ch <- prometheus.MustNewConstMetric(
				c.registrationsDesc, prometheus.GaugeValue,
				float64(count),
			)
		}
	}

	// Trunk status gauges (one metric per trunk with status label).
	if c.trunks != nil {
		for _, t := range c.trunks.GetAllTrunkStatuses() {
			val := 0.0
			if t.Status == "registered" || t.Status == "healthy" {
				val = 1.0
			}
			ch <- prometheus.MustNewConstMetric(
				c.trunkStatusDesc, prometheus.GaugeValue, val,
				fmt.Sprintf("%d", t.TrunkID), t.Name, t.Status,
			)
		}
	}

	// Call volume counters by direction.
	if c.cdrs != nil {
		counts, err := c.cdrs.CountByDirection(ctx)
		if err != nil {
			slog.Error("metrics: failed to count cdrs by direction", "error", err)
		} else {
			for _, dir := range []string{"inbound", "outbound", "internal"} {
				ch <- prometheus.MustNewConstMetric(
					c.callsTotalDesc, prometheus.CounterValue,
					float64(counts[dir]), dir,
				)
			}
		}
	}

	// RTP stats.
	if c.rtp != nil {
		ch <- prometheus.MustNewConstMetric(
			c.rtpSessionsDesc, prometheus.GaugeValue,
			float64(c.rtp.ActiveSessionCount()),
		)
		ch <- prometheus.MustNewConstMetric(
			c.rtpPacketsDesc, prometheus.CounterValue,
			float64(c.rtp.AggregatePacketsForwarded()),
		)
		ch <- prometheus.MustNewConstMetric(
			c.rtpPacketsDroppedDesc, prometheus.CounterValue,
			float64(c.rtp.AggregatePacketsDropped()),
		)
		ch <- prometheus.MustNewConstMetric(
			c.rtpBytesDesc, prometheus.CounterValue,
			float64(c.rtp.AggregateBytesForwarded()),
		)
	}

	// Voicemail message count.
	if c.voicemail != nil {
		count, err := c.voicemail.CountAll(ctx)
		if err != nil {
			slog.Error("metrics: failed to count voicemail messages", "error", err)
		} else {
			ch <- prometheus.MustNewConstMetric(
				c.voicemailMessagesDesc, prometheus.GaugeValue,
				float64(count),
			)
		}
	}

	// Uptime.
	ch <- prometheus.MustNewConstMetric(
		c.uptimeDesc, prometheus.GaugeValue,
		time.Since(c.startTime).Seconds(),
	)
}
