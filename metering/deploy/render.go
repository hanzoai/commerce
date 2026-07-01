package deploy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Tenant is the org a product is being provisioned for, plus the deploy knobs
// the control plane chooses. The defaults make a minimal, safe deployment.
type Tenant struct {
	// Org is the tenant org slug (e.g. "acme"). It scopes the CR name, the PVC,
	// the ingress host, and — crucially — the COMMERCE_SERVICE_ORG the meter
	// stamps as X-Hanzo-Org so usage debits the right org's prepaid balance.
	Org string

	// Namespace the CR is applied to. Empty defaults to "hanzo" (shared
	// namespace, name-suffixed isolation). Set to a per-org namespace for
	// strong isolation.
	Namespace string

	// Tag pins the product image (immutable semver/sha). Required — floating
	// tags must never reach a cluster.
	Tag string

	// MeterProxyImage is the GHCR image of the metering sidecar. Required when
	// the product is Meterable.
	MeterProxyImage string

	// MeterProxyTag pins the sidecar image.
	MeterProxyTag string

	// CommerceURL overrides the in-cluster commerce address (rarely needed).
	CommerceURL string

	// CommerceSecret is the k8s Secret holding the commerce service token under
	// key "commerceToken" (KMS-backed). The sidecar reads it via secretKeyRef —
	// the token is NEVER inlined. Defaults to "cloud-api-secrets".
	CommerceSecret string

	// IngressHost is the public host (e.g. "acme.vector.hanzo.ai"). Empty =
	// no ingress (ClusterIP only; reached via the gateway).
	IngressHost string

	// TierAware gates on prepaid + included plan allotment (free-tier credit)
	// instead of bare prepaid balance.
	TierAware bool

	// Test routes billing to commerce's TEST ledger (staging/sandbox).
	Test bool

	// Replicas defaults to 1.
	Replicas int
}

func (t Tenant) namespace() string {
	if t.Namespace != "" {
		return t.Namespace
	}
	return "hanzo"
}

func (t Tenant) commerceSecret() string {
	if t.CommerceSecret != "" {
		return t.CommerceSecret
	}
	return "cloud-api-secrets"
}

func (t Tenant) replicas() int {
	if t.Replicas > 0 {
		return t.Replicas
	}
	return 1
}

// Render produces the operator Service CR YAML that deploys product p for
// tenant t with prepaid metering wired. The shape:
//
//   - For a Meterable product: the Service's public port is the meter-proxy
//     (METER_PROXY_LISTEN), which gates on balance then forwards to the product
//     on 127.0.0.1:<product port> in the SAME pod (a sidecar). The product
//     never sees an unmetered request.
//   - For a non-Meterable product (billing in-process, e.g. base): the product
//     container is exposed directly; the commerce env is still injected so the
//     in-process meter can bill.
//
// Every Meterable deployment carries the commerce wiring as env: COMMERCE_URL,
// the service token via secretKeyRef (KMS-backed, never inlined), and
// COMMERCE_SERVICE_ORG=<tenant org> so debits land in the tenant's namespace.
func Render(p Product, t Tenant) (string, error) {
	if strings.TrimSpace(t.Org) == "" {
		return "", fmt.Errorf("deploy: tenant Org is required")
	}
	if strings.TrimSpace(t.Tag) == "" {
		return "", fmt.Errorf("deploy: tenant Tag is required (no floating tags)")
	}
	if p.Meterable && (strings.TrimSpace(t.MeterProxyImage) == "" || strings.TrimSpace(t.MeterProxyTag) == "") {
		return "", fmt.Errorf("deploy: MeterProxyImage and MeterProxyTag are required for meterable product %q", p.Name)
	}
	if p.StorageMount != "" && p.StorageSize == "" {
		return "", fmt.Errorf("deploy: product %q sets StorageMount but no StorageSize", p.Name)
	}

	commerceURL := t.CommerceURL
	if commerceURL == "" {
		commerceURL = "http://commerce.hanzo.svc.cluster.local:8001"
	}

	data := tmplData{
		Name:           fmt.Sprintf("%s-%s", p.Name, t.Org),
		Namespace:      t.namespace(),
		Org:            t.Org,
		Product:        p.Name,
		Provider:       p.provider(),
		Unit:           string(p.Unit),
		ProductImage:   p.Image,
		ProductTag:     t.Tag,
		ProductPort:    p.Port,
		Replicas:       t.replicas(),
		Meterable:      p.Meterable,
		ProxyImage:     t.MeterProxyImage,
		ProxyTag:       t.MeterProxyTag,
		ProxyListen:    p.Port + 10000, // public port = product port + 10000
		Prices:         p.Prices,
		SkipPaths:      strings.Join(p.SkipPaths, ","),
		CommerceURL:    commerceURL,
		CommerceSecret: t.commerceSecret(),
		TierAware:      t.TierAware,
		Test:           t.Test,
		StorageMount:   p.StorageMount,
		StorageSize:    p.StorageSize,
		PVCName:        fmt.Sprintf("%s-%s-data", p.Name, t.Org),
		IngressHost:    t.IngressHost,
		Env:            sortedEnv(p.Env),
	}
	// The public service port: proxy when metered, else the product itself.
	data.ServicePort = data.ProductPort
	if p.Meterable {
		data.ServicePort = data.ProxyListen
	}

	var buf bytes.Buffer
	if err := crTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("deploy: render %q for %q: %w", p.Name, t.Org, err)
	}
	return buf.String(), nil
}

type kv struct{ Key, Value string }

func sortedEnv(m map[string]string) []kv {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// simple insertion sort to avoid importing sort for a tiny map; keeps output
	// deterministic so the same input always yields byte-identical YAML.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	out := make([]kv, 0, len(keys))
	for _, k := range keys {
		out = append(out, kv{Key: k, Value: m[k]})
	}
	return out
}

type tmplData struct {
	Name           string
	Namespace      string
	Org            string
	Product        string
	Provider       string
	Unit           string
	ProductImage   string
	ProductTag     string
	ProductPort    int
	ServicePort    int
	Replicas       int
	Meterable      bool
	ProxyImage     string
	ProxyTag       string
	ProxyListen    int
	Prices         string
	SkipPaths      string
	CommerceURL    string
	CommerceSecret string
	TierAware      bool
	Test           bool
	StorageMount   string
	StorageSize    string
	PVCName        string
	IngressHost    string
	Env            []kv
}

// crTemplate emits a hanzo.ai/v1 Service CR. When Meterable, the meter-proxy is
// the main container (terminates ServicePort) with the product as a sidecar on
// localhost; the proxy holds all commerce wiring. When not Meterable, the
// product is the main container and carries the commerce env itself.
var crTemplate = template.Must(template.New("cr").Parse(`# Generated by github.com/hanzoai/commerce/metering/deploy — do not edit by hand.
# Product: {{.Product}}  Tenant org: {{.Org}}  Billed per: {{.Unit}}
apiVersion: hanzo.ai/v1
kind: Service
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/part-of: paas
    app.kubernetes.io/component: {{.Product}}
    hanzo.ai/tenant: {{.Org}}
    hanzo.ai/metered: "{{.Meterable}}"
spec:
{{- if .Meterable}}
  image:
    repository: {{.ProxyImage}}
    tag: "{{.ProxyTag}}"
    pullPolicy: IfNotPresent
  replicas: {{.Replicas}}
  ports:
    - name: http
      containerPort: {{.ProxyListen}}
      servicePort: {{.ServicePort}}
  env:
    - name: METER_PROXY_LISTEN
      value: ":{{.ProxyListen}}"
    - name: METER_PROXY_UPSTREAM
      value: "http://127.0.0.1:{{.ProductPort}}"
    - name: METER_PROXY_PROVIDER
      value: "{{.Provider}}"
    - name: METER_PROXY_PRICES
      value: "{{.Prices}}"
    - name: METER_PROXY_SKIP
      value: "{{.SkipPaths}}"
    - name: COMMERCE_URL
      value: "{{.CommerceURL}}"
    - name: COMMERCE_SERVICE_ORG
      value: "{{.Org}}"
    - name: COMMERCE_SERVICE_TOKEN
      valueFrom:
        secretKeyRef:
          name: {{.CommerceSecret}}
          key: commerceToken
{{- if .TierAware}}
    - name: METERING_TIER_AWARE
      value: "true"
{{- end}}
{{- if .Test}}
    - name: METERING_TEST
      value: "true"
{{- end}}
  sidecars:
    # The product runs as a sidecar; containers share the pod network namespace,
    # so the proxy reaches it on 127.0.0.1:{{.ProductPort}} with no port mapping
    # needed. (The operator Container schema has no ports field — only the main
    # container's ports become the Service.)
    - name: {{.Product}}
      image: {{.ProductImage}}:{{.ProductTag}}
{{- if .Env}}
      env:
{{- range .Env}}
        - name: {{.Key}}
          value: "{{.Value}}"
{{- end}}
{{- end}}
{{- if .StorageMount}}
      volumeMounts:
        - name: data
          mountPath: {{.StorageMount}}
{{- end}}
{{- else}}
  image:
    repository: {{.ProductImage}}
    tag: "{{.ProductTag}}"
    pullPolicy: IfNotPresent
  replicas: {{.Replicas}}
  ports:
    - name: http
      containerPort: {{.ProductPort}}
      servicePort: {{.ServicePort}}
  env:
    - name: COMMERCE_URL
      value: "{{.CommerceURL}}"
    - name: COMMERCE_SERVICE_ORG
      value: "{{.Org}}"
    - name: COMMERCE_SERVICE_TOKEN
      valueFrom:
        secretKeyRef:
          name: {{.CommerceSecret}}
          key: commerceToken
{{- if .TierAware}}
    - name: METERING_TIER_AWARE
      value: "true"
{{- end}}
{{- if .Test}}
    - name: METERING_TEST
      value: "true"
{{- end}}
{{- range .Env}}
    - name: {{.Key}}
      value: "{{.Value}}"
{{- end}}
{{- if .StorageMount}}
  volumeMounts:
    - name: data
      mountPath: {{.StorageMount}}
{{- end}}
{{- end}}
{{- if .StorageMount}}
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: {{.PVCName}}
{{- end}}
  imagePullSecrets:
    - name: ghcr-secret
{{- if .IngressHost}}
  ingress:
    enabled: true
    hosts:
      - {{.IngressHost}}
    tls: true
{{- else}}
  ingress:
    enabled: false
{{- end}}
`))
