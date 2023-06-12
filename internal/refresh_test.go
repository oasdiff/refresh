package internal_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oasdiff/go-common/ds"
	"github.com/oasdiff/go-common/gcs"
	"github.com/oasdiff/refresh/internal"
	"github.com/stretchr/testify/require"
	"github.com/tufin/oasdiff/checker"
	"gopkg.in/yaml.v3"
)

func TestRun(t *testing.T) {

	const tenant, oldSpecFileName = "tenant-1", "1234567"
	specServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newSpec, err := ioutil.ReadFile("../data/openapi-test3.yaml")
		require.NoError(t, err)
		_, err = w.Write(newSpec)
		require.NoError(t, err)
	}))

	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var breakingChanges map[string][]checker.BackwardCompatibilityError
		require.NoError(t, yaml.NewDecoder(r.Body).Decode(&breakingChanges))
		require.Len(t, breakingChanges["breaking-changes"], 6)
	}))

	oldSpec, err := ioutil.ReadFile("../data/openapi-test1.yaml")
	require.NoError(t, err)
	store := gcs.NewInMemoryStore(map[string][]byte{
		fmt.Sprintf("%s/spec/%s", tenant, oldSpecFileName): oldSpec,
	})

	require.NoError(t, internal.Run(ds.NewInMemoryClient(map[ds.Kind]interface{}{
		ds.KindWebhook: []ds.Webhook{{
			TenantId: tenant,
			Callback: callbackServer.URL,
			Spec:     specServer.URL,
			Copy:     oldSpecFileName,
			Created:  time.Now().Unix(),
		}}}), store))
}
