package records

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSummarize(t *testing.T) {
	t0, err := time.Parse(time.RFC3339, "2021-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		records         EventRecords
		now             time.Time
		expectedSummary EventSummary
	}{
		"empty": {
			records:         EventRecords{},
			expectedSummary: EventSummary{},
		},
		"missing down0": {
			records: EventRecords{
				UpEvents: []UpEvent{
					{Up: true, Timestamp: t0},
				},
			},
			expectedSummary: EventSummary{},
		},
		"not up yet": {
			records: EventRecords{
				// up:
				// down:   _____
				// event:  0   1
				// hrs:      1
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
				},
			},
			now: t0.Add(time.Hour),
			expectedSummary: EventSummary{
				DownTime: time.Hour,
			},
		},
		"just up": {
			records: EventRecords{
				// up:
				// down:   ____|
				// event:  0   1
				// hrs:      1
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					{Up: true, Timestamp: t0.Add(time.Hour)},
				},
			},
			now: t0.Add(time.Hour),
			expectedSummary: EventSummary{
				DownTime:            time.Hour,
				DownTimeProvisioned: time.Hour,
			},
		},
		"up for 3 hours": {
			records: EventRecords{
				// up:         _____
				// down:   ____|
				// event:  0   1   2
				// hrs:      1   3
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					{Up: true, Timestamp: t0.Add(time.Hour)},
				},
			},
			now: t0.Add(time.Hour + 3*time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned: time.Hour,
				DownTime:            time.Hour,
				UpTime:              3 * time.Hour,
			},
		},
		"single interruption": {
			records: EventRecords{
				// up:         _____
				// down:   ____|   |
				// event:  0   1   2
				// hrs:      1   1
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
				},
			},
			now: t0.Add(2 * time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          time.Hour,
				DownTime:                        time.Hour,
				InterruptionCount:               1,
				TotalUpTimeBetweenInterruption:  time.Hour,
				MeanUpTimeBetweenInterruption:   time.Hour,
				LatestUpTimeBetweenInterruption: time.Hour,
			},
		},
		"single interruption then down for an hour": {
			records: EventRecords{
				// up:         _____
				// down:   ____|   |____
				// event:  0   1   2
				// hrs:      1   1   1
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
				},
			},
			now: t0.Add(2*time.Hour + time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          time.Hour,
				DownTime:                        2 * time.Hour,
				InterruptionCount:               1,
				TotalUpTimeBetweenInterruption:  time.Hour,
				MeanUpTimeBetweenInterruption:   time.Hour,
				LatestUpTimeBetweenInterruption: time.Hour,
			},
		},
		"single interruption single recovery": {
			// up:         _____
			// down:   ____|   |___|
			// event:  0   1   2   3
			// hrs:      1   1   1
			records: EventRecords{
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 hr of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
					// 1 hr of downtime before recovery 0
					{Up: true, Timestamp: t0.Add(3 * time.Hour)},
				},
			},
			now: t0.Add(3 * time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          time.Hour,
				DownTime:                        2 * time.Hour,
				InterruptionCount:               1,
				RecoveryCount:                   1,
				TotalDownTimeBetweenRecovery:    time.Hour,
				MeanDownTimeBetweenRecovery:     time.Hour,
				LatestDownTimeBetweenRecovery:   time.Hour,
				TotalUpTimeBetweenInterruption:  time.Hour,
				MeanUpTimeBetweenInterruption:   time.Hour,
				LatestUpTimeBetweenInterruption: time.Hour,
			},
		},
		"single interruption single recovery then up for an hour": {
			// up:         _____   _____
			// down:   ____|   |___|
			// event:  0   1   2   3
			// hrs:      1   1   1   1
			records: EventRecords{
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 hr of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
					// 1 hr of downtime before recovery 0
					{Up: true, Timestamp: t0.Add(3 * time.Hour)},
				},
			},
			now: t0.Add(3*time.Hour + time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          2 * time.Hour,
				DownTime:                        2 * time.Hour,
				InterruptionCount:               1,
				RecoveryCount:                   1,
				TotalDownTimeBetweenRecovery:    time.Hour,
				MeanDownTimeBetweenRecovery:     time.Hour,
				LatestDownTimeBetweenRecovery:   time.Hour,
				TotalUpTimeBetweenInterruption:  time.Hour,
				MeanUpTimeBetweenInterruption:   time.Hour,
				LatestUpTimeBetweenInterruption: time.Hour,
			},
		},
		"two interruptions single recovery": {
			// up:         _____   _____
			// down:   ____|   |___|   |
			// event:  0   1   2   3   4
			// hrs:      1   1   1   2
			records: EventRecords{
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 hr of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
					// 1 hr of downtime before recovery 0
					{Up: true, Timestamp: t0.Add(3 * time.Hour)},
					// 2 hrs of uptime before interruption 1
					{Up: false, Timestamp: t0.Add(3*time.Hour + 2*time.Hour)},
				},
			},
			now: t0.Add(3*time.Hour + 2*time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          time.Hour + 2*time.Hour,
				DownTime:                        2 * time.Hour,
				InterruptionCount:               2,
				RecoveryCount:                   1,
				TotalDownTimeBetweenRecovery:    time.Hour,
				MeanDownTimeBetweenRecovery:     time.Hour,
				LatestDownTimeBetweenRecovery:   time.Hour,
				TotalUpTimeBetweenInterruption:  1*time.Hour + 2*time.Hour,
				MeanUpTimeBetweenInterruption:   (1*time.Hour + 2*time.Hour) / 2,
				LatestUpTimeBetweenInterruption: 2 * time.Hour,
			},
		},
		"two interruptions one recovery": {
			// up:         _____   _____
			// down:   ____|   |___|   |___
			// event:  0   1   2   3   4
			// hrs:      1   1   1   2   3
			records: EventRecords{
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 hr of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
					// 1 hr of downtime before recovery 0
					{Up: true, Timestamp: t0.Add(3 * time.Hour)},
					// 2 hrs of uptime before interruption 1
					{Up: false, Timestamp: t0.Add(3*time.Hour + 2*time.Hour)},
				},
			},
			now: t0.Add(3*time.Hour + 2*time.Hour + 3*time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          time.Hour + 2*time.Hour,
				DownTime:                        time.Hour + time.Hour + 3*time.Hour,
				InterruptionCount:               2,
				RecoveryCount:                   1,
				TotalDownTimeBetweenRecovery:    1 * time.Hour,
				MeanDownTimeBetweenRecovery:     1 * time.Hour,
				LatestDownTimeBetweenRecovery:   1 * time.Hour,
				TotalUpTimeBetweenInterruption:  1*time.Hour + 2*time.Hour,
				MeanUpTimeBetweenInterruption:   (1*time.Hour + 2*time.Hour) / 2,
				LatestUpTimeBetweenInterruption: 2 * time.Hour,
			},
		},
		"two interruptions two recoveries": {
			// up:         _____   _____
			// down:   ____|   |___|   |___|
			// event:  0   1   2   3   4   5
			// hrs:      1   1   1   2   3
			records: EventRecords{
				UpEvents: []UpEvent{
					{Up: false, Timestamp: t0},
					// 1 hr to come up
					{Up: true, Timestamp: t0.Add(time.Hour)},
					// 1 hr of uptime before interruption 0
					{Up: false, Timestamp: t0.Add(2 * time.Hour)},
					// 1 hr of downtime before recovery 0
					{Up: true, Timestamp: t0.Add(3 * time.Hour)},
					// 2 hrs of uptime before interruption 1
					{Up: false, Timestamp: t0.Add(3*time.Hour + 2*time.Hour)},
					// 3 hrs of downtime before recovery 1
					{Up: true, Timestamp: t0.Add(3*time.Hour + 2*time.Hour + 3*time.Hour)},
				},
			},
			now: t0.Add(3*time.Hour + 2*time.Hour + 3*time.Hour),
			expectedSummary: EventSummary{
				DownTimeProvisioned:             time.Hour,
				UpTime:                          time.Hour + 2*time.Hour,
				DownTime:                        time.Hour + time.Hour + 3*time.Hour,
				InterruptionCount:               2,
				RecoveryCount:                   2,
				TotalDownTimeBetweenRecovery:    time.Hour + 3*time.Hour,
				MeanDownTimeBetweenRecovery:     2 * time.Hour,
				LatestDownTimeBetweenRecovery:   3 * time.Hour,
				TotalUpTimeBetweenInterruption:  1*time.Hour + 2*time.Hour,
				MeanUpTimeBetweenInterruption:   (1*time.Hour + 2*time.Hour) / 2,
				LatestUpTimeBetweenInterruption: 2 * time.Hour,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotSum := tc.records.Summarize(tc.now)
			require.Equal(t, tc.expectedSummary.UpTime, gotSum.UpTime, "UpTime")
			require.Equal(t, tc.expectedSummary.DownTime, gotSum.DownTime, "DownTime")
			require.Equal(t, tc.expectedSummary.InterruptionCount, gotSum.InterruptionCount, "InterruptionCount")
			require.Equal(t, tc.expectedSummary.RecoveryCount, gotSum.RecoveryCount, "RecoveryCount")
			require.Equal(t, tc.expectedSummary.TotalDownTimeBetweenRecovery, gotSum.TotalDownTimeBetweenRecovery, "TotalDownTimeBetweenRecovery")
			require.Equal(t, tc.expectedSummary.MeanDownTimeBetweenRecovery, gotSum.MeanDownTimeBetweenRecovery, "MeanDownTimeBetweenRecovery")
			require.Equal(t, tc.expectedSummary.LatestDownTimeBetweenRecovery, gotSum.LatestDownTimeBetweenRecovery, "LatestDownTimeBetweenRecovery")
			require.Equal(t, tc.expectedSummary.TotalUpTimeBetweenInterruption, gotSum.TotalUpTimeBetweenInterruption, "TotalUpTimeBetweenInterruption")
			require.Equal(t, tc.expectedSummary.MeanUpTimeBetweenInterruption, gotSum.MeanUpTimeBetweenInterruption, "MeanUpTimeBetweenInterruption")
			require.Equal(t, tc.expectedSummary.LatestUpTimeBetweenInterruption, gotSum.LatestUpTimeBetweenInterruption, "LatestUpTimeBetweenInterruption")
		})
	}
}
