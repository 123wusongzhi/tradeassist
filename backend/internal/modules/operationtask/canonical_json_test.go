package operationtask_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
)

func TestComputePayloadHashCanonicalJSON(t *testing.T) {
	a, err := operationtask.ComputePayloadHash([]byte(`{"b":2,"a":1,"nested":{"z":true,"a":null}}`))
	require.NoError(t, err)
	b, err := operationtask.ComputePayloadHash([]byte("{\n  \"nested\": {\"a\": null, \"z\": true}, \"a\": 1.0, \"b\": 2e0\n}"))
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Equal(t, 1, operationtask.CanonicalJSONHashVersion)
	require.True(t, regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a))
}

func TestComputePayloadHashDistinguishesArraysAndValues(t *testing.T) {
	one, err := operationtask.ComputePayloadHash([]byte(`{"items":[1,2,3],"ok":true}`))
	require.NoError(t, err)
	two, err := operationtask.ComputePayloadHash([]byte(`{"items":[3,2,1],"ok":true}`))
	require.NoError(t, err)
	three, err := operationtask.ComputePayloadHash([]byte(`{"items":[1,2,3],"ok":false}`))
	require.NoError(t, err)
	require.NotEqual(t, one, two)
	require.NotEqual(t, one, three)
}

func TestComputePayloadHashNumberCanonicalization(t *testing.T) {
	one, err := operationtask.ComputePayloadHash([]byte(`{"n":1}`))
	require.NoError(t, err)
	two, err := operationtask.ComputePayloadHash([]byte(`{"n":1.0}`))
	require.NoError(t, err)
	three, err := operationtask.ComputePayloadHash([]byte(`{"n":1e0}`))
	require.NoError(t, err)
	require.Equal(t, one, two)
	require.Equal(t, one, three)

	decimal, err := operationtask.CanonicalizeJSON([]byte(`{"n":123.4500e-2}`))
	require.NoError(t, err)
	require.Equal(t, `{"n":1.2345}`, string(decimal))
}

func TestComputePayloadHashRejectsInvalidJSON(t *testing.T) {
	_, err := operationtask.ComputePayloadHash([]byte(`{"a":1`))
	require.ErrorIs(t, err, operationtask.ErrValidation)
	_, err = operationtask.ComputePayloadHash([]byte(`{"a":1}{"b":2}`))
	require.ErrorIs(t, err, operationtask.ErrValidation)
}
