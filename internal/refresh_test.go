package internal_test

import (
	"encoding/json"
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
)

func TestRun(t *testing.T) {

	const tenantId, webhookName, oldSpecFileName = "tenant-1", "Andes", "1234567"
	specServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newSpec, err := ioutil.ReadFile("../data/openapi-test3.yaml")
		require.NoError(t, err)
		_, err = w.Write(newSpec)
		require.NoError(t, err)
	}))

	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var webhooks map[string][]*internal.WebhookBreakingChanges
		require.NoError(t, json.NewDecoder(r.Body).Decode(&webhooks))
		require.NoError(t, r.Body.Close())
		require.Len(t, webhooks, 1)
		payload := webhooks["webhooks"]
		require.Len(t, payload, 1)
		require.Equal(t, webhookName, payload[0].Name)
		require.Len(t, payload[0].BreakingChanges, 6)
	}))

	oldSpec, err := ioutil.ReadFile("../data/openapi-test1.yaml")
	require.NoError(t, err)
	store := gcs.NewInMemoryStore(map[string][]byte{
		fmt.Sprintf("%s/spec/%s", tenantId, oldSpecFileName): oldSpec,
	})

	require.NoError(t, internal.Run(ds.NewInMemoryClient(map[ds.Kind]interface{}{
		ds.KindTenant: []ds.Tenant{{
			Id:           tenantId,
			Name:         "my-company",
			Email:        "john@my-company.com",
			Callback:     callbackServer.URL,
			SlackChannel: "",
			Created:      time.Now().Unix(),
		}},
		ds.KindWebhook: []ds.Webhook{{
			Name:     webhookName,
			TenantId: tenantId,
			Spec:     specServer.URL,
			Copy:     oldSpecFileName,
			Created:  time.Now().Unix(),
		}}}), store))
}
