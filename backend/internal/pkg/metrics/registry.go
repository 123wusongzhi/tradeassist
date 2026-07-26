package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
)

// Registry wraps prometheus registry with cardinality validation.
type Registry struct {
	prom      *prometheus.Registry
	validator *LabelValidator
	mu        sync.Mutex
	counters  map[string]*prometheus.CounterVec
	gauges    map[string]*prometheus.GaugeVec
	histogram map[string]*prometheus.HistogramVec
}

// NewRegistry creates an isolated metrics registry.
func NewRegistry(namespace string) *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if namespace != "" {
		reg.MustRegister(prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "build_info",
			Help:      "TradeMind build info",
			ConstLabels: prometheus.Labels{
				"service": namespace,
			},
		}))
	}
	return &Registry{
		prom:      reg,
		validator: NewLabelValidator(0),
		counters:  make(map[string]*prometheus.CounterVec),
		gauges:    make(map[string]*prometheus.GaugeVec),
		histogram: make(map[string]*prometheus.HistogramVec),
	}
}

// Prometheus returns underlying prometheus registry.
func (r *Registry) Prometheus() *prometheus.Registry {
	if r == nil {
		return prometheus.NewRegistry()
	}
	return r.prom
}

// Validator returns label validator.
func (r *Registry) Validator() *LabelValidator {
	if r == nil {
		return NewLabelValidator(0)
	}
	return r.validator
}

func (r *Registry) labelNames(labels ...string) ([]string, error) {
	for _, l := range labels {
		if err := r.validator.Validate(l, "init"); err != nil {
			return nil, err
		}
	}
	return labels, nil
}

// Counter returns or creates a counter vec.
func (r *Registry) Counter(name, help string, labels ...string) (*prometheus.CounterVec, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.counters[name]; ok {
		return existing, nil
	}
	labelNames, err := func() ([]string, error) {
		for _, l := range labels {
			if err := r.validator.ValidateKey(l); err != nil {
				return nil, err
			}
		}
		return labels, nil
	}()
	if err != nil {
		return nil, err
	}
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, labelNames)
	if err := r.prom.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				r.counters[name] = existing
				return existing, nil
			}
		}
		return nil, err
	}
	r.counters[name] = vec
	return vec, nil
}

// Gauge returns or creates a gauge vec.
func (r *Registry) Gauge(name, help string, labels ...string) (*prometheus.GaugeVec, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.gauges[name]; ok {
		return existing, nil
	}
	labelNames, err := func() ([]string, error) {
		for _, l := range labels {
			if err := r.validator.ValidateKey(l); err != nil {
				return nil, err
			}
		}
		return labels, nil
	}()
	if err != nil {
		return nil, err
	}
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, labelNames)
	if err := r.prom.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				r.gauges[name] = existing
				return existing, nil
			}
		}
		return nil, err
	}
	r.gauges[name] = vec
	return vec, nil
}

// Histogram returns or creates a histogram vec.
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) (*prometheus.HistogramVec, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.histogram[name]; ok {
		return existing, nil
	}
	labelNames, err := func() ([]string, error) {
		for _, l := range labels {
			if err := r.validator.ValidateKey(l); err != nil {
				return nil, err
			}
		}
		return labels, nil
	}()
	if err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}
	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	}, labelNames)
	if err := r.prom.Register(vec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				r.histogram[name] = existing
				return existing, nil
			}
		}
		return nil, err
	}
	r.histogram[name] = vec
	return vec, nil
}

// IncCounter safely increments a counter with validated label values.
func (r *Registry) IncCounter(name string, labelValues map[string]string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	vec, ok := r.counters[name]
	r.mu.Unlock()
	if !ok || vec == nil {
		return
	}
	vals := make([]string, 0, len(labelValues))
	keys := make([]string, 0, len(labelValues))
	for k, v := range labelValues {
		if err := r.validator.Validate(k, v); err != nil {
			return
		}
		keys = append(keys, k)
		vals = append(vals, v)
	}
	vec.WithLabelValues(vals...).Inc()
}

// SnapshotValues returns aggregate metric samples by metric name.
// It intentionally drops label dimensions so alert/SLO evaluators cannot depend
// on high-cardinality data or expose raw tenant/shop/task identifiers.
func (r *Registry) SnapshotValues() map[string]float64 {
	out := make(map[string]float64)
	if r == nil || r.prom == nil {
		return out
	}
	families, err := r.prom.Gather()
	if err != nil {
		return out
	}
	for _, mf := range families {
		name := mf.GetName()
		if name == "" {
			continue
		}
		for _, m := range mf.GetMetric() {
			out[name] += metricValue(m)
		}
	}
	return out
}

func metricValue(m *dto.Metric) float64 {
	if m == nil {
		return 0
	}
	switch {
	case m.Counter != nil:
		return m.Counter.GetValue()
	case m.Gauge != nil:
		return m.Gauge.GetValue()
	case m.Untyped != nil:
		return m.Untyped.GetValue()
	case m.Histogram != nil:
		return float64(m.Histogram.GetSampleCount())
	case m.Summary != nil:
		return float64(m.Summary.GetSampleCount())
	default:
		return 0
	}
}
