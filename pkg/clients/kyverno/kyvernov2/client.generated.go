package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2"
	policyexceptions "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/policyexceptions"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.PolicyExceptionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyException", c.clientType)
	return policyexceptions.WithMetrics(c.inner.PolicyExceptions(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.PolicyExceptionInterface {
	return policyexceptions.WithTracing(c.inner.PolicyExceptions(namespace), c.client, "PolicyException")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.PolicyExceptionInterface {
	return policyexceptions.WithLogging(c.inner.PolicyExceptions(namespace), c.logger.WithValues("resource", "PolicyExceptions").WithValues("namespace", namespace))
}
