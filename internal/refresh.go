package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/go-common/ds"
	"github.com/oasdiff/go-common/gcs"
	"github.com/sirupsen/logrus"
	"github.com/tufin/oasdiff/checker"
	"github.com/tufin/oasdiff/diff"
	"github.com/tufin/oasdiff/load"
)

type WebhookBreakingChanges struct {
	Name            string                               `json:"name"`
	BreakingChanges []checker.BackwardCompatibilityError `json:"breaking_changes"`
}

func Run(dsc ds.Client, store gcs.Client) error {

	tenantIdToTenant, err := getTenants(dsc)
	if err != nil {
		return err
	}

	var webhooks []ds.Webhook
	if err := dsc.GetAll(ds.KindWebhook, &webhooks); err != nil {
		return err
	}

	for _, currWebhook := range webhooks {
		oldSpec, err := loadSpecFromStorage(store, currWebhook.TenantId, currWebhook.Copy)
		if err != nil {
			continue
		}
		newSpec, err := loadSpecFromUri(currWebhook.TenantId, currWebhook.Spec)
		if err != nil {
			continue
		}

		breakingChanges, err := checkBreaking(oldSpec, currWebhook.Copy, newSpec, currWebhook.Spec)
		if err != nil {
			continue
		}
		if len(breakingChanges) > 0 {
			tenant, ok := tenantIdToTenant[currWebhook.TenantId]
			if !ok {
				logrus.Errorf("tenant not found for webhook '%s'", currWebhook.TenantId)
				continue
			}
			_ = notify(tenant, currWebhook.Name, breakingChanges)
		}
	}

	return nil
}

func getTenants(dsc ds.Client) (map[string]ds.Tenant, error) {

	var tenants []ds.Tenant
	if err := dsc.GetAll(ds.KindTenant, &tenants); err != nil {
		return nil, err
	}

	return toTenantIdToTenant(tenants), nil
}

func toTenantIdToTenant(tenants []ds.Tenant) map[string]ds.Tenant {

	res := make(map[string]ds.Tenant, len(tenants))
	for _, currTenant := range tenants {
		res[currTenant.Id] = currTenant
	}

	return res
}

func notify(tenant ds.Tenant, webhookName string, changes []checker.BackwardCompatibilityError) error {

	if tenant.Callback != "" {
		payload, err := json.Marshal(map[string][]WebhookBreakingChanges{
			"webhooks": {{Name: webhookName, BreakingChanges: changes}}})
		if err != nil {
			logrus.Errorf("[notify] failed to json marshal 'WebhookBreakingChanges' with '%v' tenant '%s', webhook '%s'", err, tenant.Id, webhookName)
			return err
		}
		response, err := http.Post(tenant.Callback, "application/json", bytes.NewBuffer(payload))
		if err != nil {
			logrus.Errorf("[notify] failed to send 'WebhookBreakingChanges' json with '%v' tenant '%s', webhook '%s'", err, tenant.Id, webhookName)
			return err
		}
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
			err := fmt.Errorf("[notify] failed to send 'WebhookBreakingChanges' report to webhook with '%s' tenant '%s', webhook '%s'", response.Status, tenant.Id, webhookName)
			logrus.Info(err.Error())
			return err
		}
	}

	return nil
}

func checkBreaking(oldSpec *openapi3.T, oldSpecPath string, newSpec *openapi3.T, newSpecPath string) ([]checker.BackwardCompatibilityError, error) {

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	diffReport, operationsSources, err := diff.GetWithOperationsSourcesMap(
		diff.NewConfig().WithCheckBreaking(),
		&load.SpecInfo{
			Url:  oldSpecPath,
			Spec: oldSpec,
		}, &load.SpecInfo{
			Url:  newSpecPath,
			Spec: newSpec,
		})
	if err != nil {
		logrus.Errorf("failed to get diff with '%v'", err)
		return nil, err
	}

	return checker.CheckBackwardCompatibility(checker.GetDefaultChecks(),
		diffReport, operationsSources), nil
}

func loadSpecFromStorage(store gcs.Client, tenant string, name string) (*openapi3.T, error) {

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	path := fmt.Sprintf("%s/spec/%s", tenant, name)
	spec, err := store.Read(path)
	if err != nil {
		return nil, err
	}
	res, err := loader.LoadFromData(spec)
	if err != nil {
		logrus.Errorf("failed to load spec '%s' from gcs with '%v'", path, err)
		return nil, err
	}

	return res, nil
}

func loadSpecFromUri(tenant string, specUrl string) (*openapi3.T, error) {

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	u, err := url.ParseRequestURI(specUrl)
	if err != nil {
		logrus.Infof("invalid spec url '%s' with '%v' tenant '%s'", specUrl, err, tenant)
		return nil, err
	}

	t, err := loader.LoadFromURI(u)
	if err != nil {
		logrus.Infof("failed to load OpenAPI spec from '%s' with '%v' tenant '%s'", specUrl, err, tenant)
		return nil, err
	}

	// lint
	// err = t.Validate(context.Background())
	// if err != nil {
	// 	logrus.Infof("failed to validate OpenAPI spec from '%s' with '%v' tenant '%s'", specUrl, err, tenant)
	// 	return nil, err
	// }

	return t, nil
}
