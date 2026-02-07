package scheduler

import (
"testing"
"time"
)

func TestGenerateDynamicWindow(t *testing.T) {
// Test window generation
rangeStart := 8 * time.Hour       // 08:00
rangeEnd := 18 * time.Hour        // 18:00
windowSpan := 45 * time.Minute

seed := time.Now().Unix() / 86400
start, end := GenerateDynamicWindow(rangeStart, rangeEnd, windowSpan, seed)

// Parse the generated times
startDur, err := ParseTimeWindow(start)
if err != nil {
t.Fatalf("Failed to parse start time: %v", err)
}
endDur, err := ParseTimeWindow(end)
if err != nil {
t.Fatalf("Failed to parse end time: %v", err)
}

// Verify window is within range
if startDur < rangeStart {
t.Errorf("Window start %s is before range start", start)
}
if endDur > rangeEnd {
t.Errorf("Window end %s is after range end", end)
}

// Verify window span is correct
actualSpan := endDur - startDur
if actualSpan != windowSpan {
t.Errorf("Window span %v does not match expected %v", actualSpan, windowSpan)
}

// Test that same seed produces same window
start2, end2 := GenerateDynamicWindow(rangeStart, rangeEnd, windowSpan, seed)
if start != start2 || end != end2 {
t.Errorf("Same seed produced different windows: %s-%s vs %s-%s", start, end, start2, end2)
}

t.Logf("Generated window: %s - %s", start, end)
}

func TestGenerateRandomDelay(t *testing.T) {
windowSpan := 45 * time.Minute

// Generate multiple delays and verify they're within the window
delays := make([]time.Duration, 10)
allSame := true
for i := 0; i < 10; i++ {
delay := GenerateRandomDelay(windowSpan)
delays[i] = delay

if delay < 0 || delay > windowSpan {
t.Errorf("Random delay %v is outside window span %v", delay, windowSpan)
}

if i > 0 && delay != delays[0] {
allSame = false
}

// Small sleep to ensure different random seeds
time.Sleep(time.Millisecond)
}

// Verify that we got at least some variation (not all the same)
if allSame {
t.Logf("Warning: All delays were the same: %v", delays[0])
} else {
t.Logf("Generated varying delays: %v", delays)
}
}

func TestIsWithinWindow(t *testing.T) {
tests := []struct {
name     string
hour     int
minute   int
start    time.Duration
end      time.Duration
expected bool
}{
{
name:     "Within window",
hour:     10,
minute:   0,
start:    8 * time.Hour,
end:      12 * time.Hour,
expected: true,
},
{
name:     "Before window",
hour:     7,
minute:   0,
start:    8 * time.Hour,
end:      12 * time.Hour,
expected: false,
},
{
name:     "After window",
hour:     13,
minute:   0,
start:    8 * time.Hour,
end:      12 * time.Hour,
expected: false,
},
{
name:     "At window start",
hour:     8,
minute:   0,
start:    8 * time.Hour,
end:      12 * time.Hour,
expected: true,
},
{
name:     "At window end",
hour:     12,
minute:   0,
start:    8 * time.Hour,
end:      12 * time.Hour,
expected: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
now := time.Date(2024, 1, 1, tt.hour, tt.minute, 0, 0, time.UTC)
result := IsWithinWindow(now, tt.start, tt.end)
if result != tt.expected {
t.Errorf("IsWithinWindow(%02d:%02d, %v, %v) = %v, want %v",
tt.hour, tt.minute, tt.start, tt.end, result, tt.expected)
}
})
}
}

func TestFormatWindow(t *testing.T) {
tests := []struct {
duration time.Duration
expected string
}{
{8*time.Hour + 30*time.Minute, "08:30"},
{12 * time.Hour, "12:00"},
{0, "00:00"},
{23*time.Hour + 59*time.Minute, "23:59"},
}

for _, tt := range tests {
result := FormatWindow(tt.duration)
if result != tt.expected {
t.Errorf("FormatWindow(%v) = %s, want %s", tt.duration, result, tt.expected)
}
}
}

func TestParseTimeWindow(t *testing.T) {
tests := []struct {
input    string
expected time.Duration
hasError bool
}{
{"08:30", 8*time.Hour + 30*time.Minute, false},
{"12:00", 12 * time.Hour, false},
{"00:00", 0, false},
{"23:59", 23*time.Hour + 59*time.Minute, false},
{"invalid", 0, true},
{"25:00", 0, true},
}

for _, tt := range tests {
result, err := ParseTimeWindow(tt.input)
if tt.hasError {
if err == nil {
t.Errorf("ParseTimeWindow(%s) should have returned error", tt.input)
}
} else {
if err != nil {
t.Errorf("ParseTimeWindow(%s) returned unexpected error: %v", tt.input, err)
}
if result != tt.expected {
t.Errorf("ParseTimeWindow(%s) = %v, want %v", tt.input, result, tt.expected)
}
}
}
}
