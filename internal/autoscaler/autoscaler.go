package autoscaler

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

const (
	defaultInterval        = 30 * time.Second
	defaultScaleDownWindow = 300 * time.Second
	metricsMaxAgeSeconds   = 45
)

type Autoscaler struct {
	store    storage.Storage
	interval time.Duration
}

func NewAutoscaler(store storage.Storage, interval time.Duration) *Autoscaler {
	if interval <= 0 {
		interval = defaultInterval
	}
	return &Autoscaler{store: store, interval: interval}
}

func (a *Autoscaler) Run(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("HPA tick")
			services, err := a.store.ListServices(ctx)
			if err != nil {
				log.Printf("HPA: list services: %v", err)
				continue
			}
			for _, service := range services {
				if err := a.evaluateService(ctx, service); err != nil {
					log.Printf("HPA: evaluate service %s: %v", service.Name, err)
				}
			}
		}
	}
}

func (a *Autoscaler) evaluateService(ctx context.Context, svc *types.Service) error {
	log.Printf("HPA: evaluating %s", svc.Name)
	if svc.Autoscale == nil {
		log.Printf("HPA: service %s has no autoscale config", svc.Name)
		return nil
	}
	if svc.Autoscale.TargetCPU <= 0 {
		return fmt.Errorf("service %q has non-positive target_cpu", svc.Name)
	}
	if svc.Autoscale.Min < 0 || svc.Autoscale.Max < svc.Autoscale.Min {
		return fmt.Errorf("service %q has invalid autoscale bounds", svc.Name)
	}

	replicas, err := a.store.ListReplicasByService(ctx, svc.Name)
	if err != nil {
		return fmt.Errorf("list replicas: %w", err)
	}
	var totalUtilization float64
	var sampleCount int
	var eligibleCount int
	for _, replica := range replicas {
		if replica.Status != types.PodStatusRunning ||
			replica.Health != types.HealthStatusHealthy ||
			!replica.Ready {
			continue
		}
		eligibleCount++
		if svc.CPUMillis <= 0 {
			return fmt.Errorf("service %q has non-positive cpu_millis", svc.Name)
		}
		metric, err := a.store.GetLatestMetrics(ctx, replica.ID, metricsMaxAgeSeconds)
		if err != nil {
			return fmt.Errorf("get latest metrics for replica %q: %w", replica.ID, err)
		}
		if metric == nil || time.Since(metric.Timestamp) > metricsMaxAgeSeconds*time.Second {
			continue
		}
		totalUtilization += metric.CPUCores / (float64(svc.CPUMillis) / 1000)
		sampleCount++
	}
	if eligibleCount == 0 {
		log.Printf("HPA: service %s no eligible replicas", svc.Name)
		return nil
	}
	if sampleCount == 0 {
		log.Printf("HPA: service %s metrics empty", svc.Name)
		return nil
	}

	avgCPUPercent := totalUtilization / float64(sampleCount) * 100
	targetCPUPercent := svc.Autoscale.TargetCPU
	tolerance := svc.Autoscale.Tolerance
	if tolerance <= 0 {
		tolerance = 10
	}
	if avgCPUPercent >= targetCPUPercent*(1-tolerance/100) &&
		avgCPUPercent <= targetCPUPercent*(1+tolerance/100) {
		log.Printf("HPA: service %s within tolerance (avgCPU=%f)", svc.Name, avgCPUPercent)
		return nil
	}

	current := svc.Replicas
	desired := int(math.Ceil(float64(current) * avgCPUPercent / targetCPUPercent))
	if desired < svc.Autoscale.Min {
		desired = svc.Autoscale.Min
	}
	if desired > svc.Autoscale.Max {
		desired = svc.Autoscale.Max
	}
	if desired == current {
		log.Printf("HPA: service %s already at desired", svc.Name)
		return nil
	}
	if desired < current {
		window := time.Duration(svc.Autoscale.ScaleDownWindowSeconds) * time.Second
		if window <= 0 {
			window = defaultScaleDownWindow
		}
		key := "hpa:last_scaled_down_at:" + svc.Name
		lastScaledDown, err := a.store.GetConfigValue(ctx, key)
		if err != nil {
			return fmt.Errorf("get last scale-down time: %w", err)
		}
		if lastScaledDown != "" {
			timestamp, err := time.Parse(time.RFC3339Nano, lastScaledDown)
			if err != nil {
				return fmt.Errorf("parse last scale-down time for service %q: %w", svc.Name, err)
			}
			if time.Since(timestamp) < window {
				return nil
			}
		}
	}

	if err := a.store.UpdateServiceReplicas(ctx, svc.Name, desired); err != nil {
		return fmt.Errorf("update desired replicas: %w", err)
	}
	if desired < current {
		key := "hpa:last_scaled_down_at:" + svc.Name
		if err := a.store.SetConfigValue(ctx, key, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("persist last scale-down time: %w", err)
		}
	}
	log.Printf("HPA: service=%s current=%d desired=%d avgCPU=%.1f%%", svc.Name, current, desired, avgCPUPercent)
	return nil
}
