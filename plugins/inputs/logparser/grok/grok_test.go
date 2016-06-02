package grok

import (
	"testing"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var benchM telegraf.Metric

func BenchmarkParseLinePatternFile(b *testing.B) {
	p := &Parser{
		Patterns:           []string{"%{TEST_LOG_B}", "%{TEST_LOG_A}"},
		CustomPatternFiles: []string{"./testdata/test-patterns"},
	}
	p.Compile()

	var m telegraf.Metric
	for n := 0; n < b.N; n++ {
		m, _ = p.ParseLine(`[04/Jun/2016:12:41:45 +0100] 1.25 200 192.168.1.1 5.432µs 101`)
	}
	benchM = m
}

func BenchmarkParseLinePatternString(b *testing.B) {
	p := &Parser{
		Patterns: []string{"%{TEST_LOG_A}", "%{TEST_LOG_B}"},
		CustomPatterns: `
			DURATION %{NUMBER}[nuµm]?s
			RESPONSE_CODE %{NUMBER:response_code:tag}
			RESPONSE_TIME %{DURATION:response_time:duration}
			TEST_LOG_A %{NUMBER:myfloat:float} %{RESPONSE_CODE} %{IPORHOST:clientip} %{RESPONSE_TIME}
		`,
	}
	p.Compile()

	var m telegraf.Metric
	for n := 0; n < b.N; n++ {
		m, _ = p.ParseLine(`[04/Jun/2016:12:41:45 +0100] 1.25 200 192.168.1.1 5.432µs 101`)
	}
	benchM = m
}

func TestCompileStringAndParse(t *testing.T) {
	p := &Parser{
		Patterns: []string{"%{TEST_LOG_A}", "%{TEST_LOG_B}"},
		CustomPatterns: `
			DURATION %{NUMBER}[nuµm]?s
			RESPONSE_CODE %{NUMBER:response_code:tag}
			RESPONSE_TIME %{DURATION:response_time:duration}
			TEST_LOG_A %{NUMBER:myfloat:float} %{RESPONSE_CODE} %{IPORHOST:clientip} %{RESPONSE_TIME}
		`,
	}
	assert.NoError(t, p.Compile())

	metricA, err := p.ParseLine(`1.25 200 192.168.1.1 5.432µs`)
	require.NotNil(t, metricA)
	assert.NoError(t, err)
	assert.Equal(t,
		map[string]interface{}{
			"clientip":      "192.168.1.1",
			"myfloat":       float64(1.25),
			"response_time": int64(5432),
		},
		metricA.Fields())
	assert.Equal(t, map[string]string{"response_code": "200"}, metricA.Tags())
}

func TestCompileFileAndParse(t *testing.T) {
	p := &Parser{
		Patterns:           []string{"%{TEST_LOG_A}", "%{TEST_LOG_B}"},
		CustomPatternFiles: []string{"./testdata/test-patterns"},
	}
	assert.NoError(t, p.Compile())

	metricA, err := p.ParseLine(`[04/Jun/2016:12:41:45 +0100] 1.25 200 192.168.1.1 5.432µs 101`)
	require.NotNil(t, metricA)
	assert.NoError(t, err)
	assert.Equal(t,
		map[string]interface{}{
			"clientip":      "192.168.1.1",
			"myfloat":       float64(1.25),
			"response_time": int64(5432),
			"myint":         int64(101),
		},
		metricA.Fields())
	assert.Equal(t, map[string]string{"response_code": "200"}, metricA.Tags())
	assert.Equal(t,
		time.Date(2016, time.June, 4, 12, 41, 45, 0, time.FixedZone("foo", 60*60)).Nanosecond(),
		metricA.Time().Nanosecond())

	metricB, err := p.ParseLine(`[04/06/2016--12:41:45] 1.25 mystring dropme nomodifier`)
	require.NotNil(t, metricB)
	assert.NoError(t, err)
	assert.Equal(t,
		map[string]interface{}{
			"myfloat":    1.25,
			"mystring":   "mystring",
			"nomodifier": "nomodifier",
		},
		metricB.Fields())
	assert.Equal(t, map[string]string{}, metricB.Tags())
	assert.Equal(t,
		time.Date(2016, time.June, 4, 12, 41, 45, 0, time.FixedZone("foo", 60*60)).Nanosecond(),
		metricB.Time().Nanosecond())
}

func TestCompileNoModifiersAndParse(t *testing.T) {
	p := &Parser{
		Patterns: []string{"%{TEST_LOG_C}"},
		CustomPatterns: `
			DURATION %{NUMBER}[nuµm]?s
			TEST_LOG_C %{NUMBER:myfloat} %{NUMBER} %{IPORHOST:clientip} %{DURATION:rt}
		`,
	}
	assert.NoError(t, p.Compile())

	metricA, err := p.ParseLine(`1.25 200 192.168.1.1 5.432µs`)
	require.NotNil(t, metricA)
	assert.NoError(t, err)
	assert.Equal(t,
		map[string]interface{}{
			"clientip": "192.168.1.1",
			"myfloat":  "1.25",
			"rt":       "5.432µs",
		},
		metricA.Fields())
	assert.Equal(t, map[string]string{}, metricA.Tags())
}

func TestCompileNoNamesAndParse(t *testing.T) {
	p := &Parser{
		Patterns: []string{"%{TEST_LOG_C}"},
		CustomPatterns: `
			DURATION %{NUMBER}[nuµm]?s
			TEST_LOG_C %{NUMBER} %{NUMBER} %{IPORHOST} %{DURATION}
		`,
	}
	assert.NoError(t, p.Compile())

	metricA, err := p.ParseLine(`1.25 200 192.168.1.1 5.432µs`)
	require.Nil(t, metricA)
	assert.NoError(t, err)
}

func TestParseNoMatch(t *testing.T) {
	p := &Parser{
		Patterns:           []string{"%{TEST_LOG_A}", "%{TEST_LOG_B}"},
		CustomPatternFiles: []string{"./testdata/test-patterns"},
	}
	assert.NoError(t, p.Compile())

	metricA, err := p.ParseLine(`[04/Jun/2016:12:41:45 +0100] notnumber 200 192.168.1.1 5.432µs 101`)
	assert.NoError(t, err)
	assert.Nil(t, metricA)
}

func TestCompileErrors(t *testing.T) {
	// Compile fails because there are multiple timestamps:
	p := &Parser{
		Patterns: []string{"%{TEST_LOG_A}", "%{TEST_LOG_B}"},
		CustomPatterns: `
			TEST_LOG_A %{HTTPDATE:ts1:ts-httpd} %{HTTPDATE:ts2:ts-httpd} %{NUMBER:mynum:int}
		`,
	}
	assert.Error(t, p.Compile())

	// Compile fails because file doesn't exist:
	p = &Parser{
		Patterns:           []string{"%{TEST_LOG_A}", "%{TEST_LOG_B}"},
		CustomPatternFiles: []string{"/tmp/foo/bar/baz"},
	}
	assert.Error(t, p.Compile())
}

func TestParseErrors(t *testing.T) {
	// Parse fails because the pattern doesn't exist
	p := &Parser{
		Patterns: []string{"%{TEST_LOG_B}"},
		CustomPatterns: `
			TEST_LOG_A %{HTTPDATE:ts:ts-httpd} %{WORD:myword:int} %{}
		`,
	}
	assert.NoError(t, p.Compile())
	_, err := p.ParseLine(`[04/Jun/2016:12:41:45 +0100] notnumber 200 192.168.1.1 5.432µs 101`)
	assert.Error(t, err)

	// Parse fails because myword is not an int
	p = &Parser{
		Patterns: []string{"%{TEST_LOG_A}"},
		CustomPatterns: `
			TEST_LOG_A %{HTTPDATE:ts:ts-httpd} %{WORD:myword:int}
		`,
	}
	assert.NoError(t, p.Compile())
	_, err = p.ParseLine(`04/Jun/2016:12:41:45 +0100 notnumber`)
	assert.Error(t, err)

	// Parse fails because myword is not a float
	p = &Parser{
		Patterns: []string{"%{TEST_LOG_A}"},
		CustomPatterns: `
			TEST_LOG_A %{HTTPDATE:ts:ts-httpd} %{WORD:myword:float}
		`,
	}
	assert.NoError(t, p.Compile())
	_, err = p.ParseLine(`04/Jun/2016:12:41:45 +0100 notnumber`)
	assert.Error(t, err)

	// Parse fails because myword is not a duration
	p = &Parser{
		Patterns: []string{"%{TEST_LOG_A}"},
		CustomPatterns: `
			TEST_LOG_A %{HTTPDATE:ts:ts-httpd} %{WORD:myword:duration}
		`,
	}
	assert.NoError(t, p.Compile())
	_, err = p.ParseLine(`04/Jun/2016:12:41:45 +0100 notnumber`)
	assert.Error(t, err)

	// Parse fails because the time layout is wrong.
	p = &Parser{
		Patterns: []string{"%{TEST_LOG_A}"},
		CustomPatterns: `
			TEST_LOG_A %{HTTPDATE:ts:ts-unix} %{WORD:myword:duration}
		`,
	}
	assert.NoError(t, p.Compile())
	_, err = p.ParseLine(`04/Jun/2016:12:41:45 +0100 notnumber`)
	assert.Error(t, err)
}

func TestTsModder(t *testing.T) {
	tsm := &tsModder{}

	reftime := time.Date(2006, time.December, 1, 1, 1, 1, int(time.Millisecond), time.UTC)
	modt := tsm.tsMod(reftime)
	assert.Equal(t, reftime, modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Microsecond*1), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Microsecond*2), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Microsecond*3), modt)

	reftime = time.Date(2006, time.December, 1, 1, 1, 1, int(time.Microsecond), time.UTC)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime, modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Nanosecond*1), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Nanosecond*2), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Nanosecond*3), modt)

	reftime = time.Date(2006, time.December, 1, 1, 1, 1, int(time.Microsecond)*999, time.UTC)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime, modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Nanosecond*1), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Nanosecond*2), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Nanosecond*3), modt)

	reftime = time.Date(2006, time.December, 1, 1, 1, 1, 0, time.UTC)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime, modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Millisecond*1), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Millisecond*2), modt)
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime.Add(time.Millisecond*3), modt)

	reftime = time.Time{}
	modt = tsm.tsMod(reftime)
	assert.Equal(t, reftime, modt)
}
