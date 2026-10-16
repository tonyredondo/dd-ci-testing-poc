package minitracer

import "reflect"

func (m metaSet) asMap() map[string]string {
	out := make(map[string]string, m.len())
	for k, v := range m.all {
		out[k] = v
	}
	return out
}

func (m metricSet) asMap() map[string]float64 {
	out := make(map[string]float64, m.len())
	for k, v := range m.all {
		out[k] = v
	}
	return out
}

func metaOf(values map[string]string) metaSet {
	out := metaSet{newEntrySet[string](len(values))}
	for k, v := range values {
		out.set(k, v)
	}
	return out
}

func metricsOf(values map[string]float64) metricSet {
	var out metricSet
	for k, v := range values {
		out.set(k, v)
	}
	return out
}

// sameContent compares events regardless of the order of their entries,
// which projection takes from a map.
func sameContent(a, b eventContent) bool {
	if !reflect.DeepEqual(a.Meta.asMap(), b.Meta.asMap()) || !reflect.DeepEqual(a.Metrics.asMap(), b.Metrics.asMap()) {
		return false
	}
	a.Meta, a.Metrics, b.Meta, b.Metrics = metaSet{}, metricSet{}, metaSet{}, metricSet{}
	return reflect.DeepEqual(a, b)
}
