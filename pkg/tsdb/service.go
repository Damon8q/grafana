package tsdb

import (
	"context"
	"fmt"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins/manager"
	pluginmodels "github.com/grafana/grafana/pkg/plugins/models"
	"github.com/grafana/grafana/pkg/registry"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/azuremonitor"
	"github.com/grafana/grafana/pkg/tsdb/cloudmonitoring"
	"github.com/grafana/grafana/pkg/tsdb/cloudwatch"
	"github.com/grafana/grafana/pkg/tsdb/elasticsearch"
	"github.com/grafana/grafana/pkg/tsdb/graphite"
	"github.com/grafana/grafana/pkg/tsdb/influxdb"
	"github.com/grafana/grafana/pkg/tsdb/mssql"
	"github.com/grafana/grafana/pkg/tsdb/mysql"
	"github.com/grafana/grafana/pkg/tsdb/opentsdb"
	"github.com/grafana/grafana/pkg/tsdb/postgres"
	"github.com/grafana/grafana/pkg/tsdb/prometheus"
)

// NewService returns a new Service.
func NewService() Service {
	return Service{
		registry: map[string]func(*models.DataSource) (pluginmodels.DataPlugin, error){},
	}
}

func init() {
	svc := NewService()
	registry.Register(&registry.Descriptor{
		Name:     "DataService",
		Instance: &svc,
	})
}

// Service handles data requests to data sources.
type Service struct {
	Cfg                    *setting.Cfg                  `inject:""`
	CloudWatchService      *cloudwatch.CloudWatchService `inject:""`
	PostgresService        *postgres.PostgresService     `inject:""`
	CloudMonitoringService *cloudmonitoring.Service      `inject:""`
	AzureMonitorService    *azuremonitor.Service         `inject:""`
	PluginManager          *plugins.PluginManager        `inject:""`

	registry map[string]func(*models.DataSource) (pluginmodels.DataPlugin, error)
}

// Init initialises the service.
func (s *Service) Init() error {
	s.registry["graphite"] = graphite.NewExecutor
	s.registry["opentsdb"] = opentsdb.NewExecutor
	s.registry["prometheus"] = prometheus.NewExecutor
	s.registry["influxdb"] = influxdb.NewExecutor
	s.registry["mssql"] = mssql.NewExecutor
	s.registry["postgres"] = s.PostgresService.NewExecutor
	s.registry["mysql"] = mysql.NewExecutor
	s.registry["elasticsearch"] = elasticsearch.NewExecutor
	s.registry["cloudwatch"] = s.CloudWatchService.NewExecutor
	s.registry["stackdriver"] = s.CloudMonitoringService.NewExecutor
	s.registry["grafana-azure-monitor-datasource"] = s.AzureMonitorService.NewExecutor
	return nil
}

func (s *Service) HandleRequest(ctx context.Context, ds *models.DataSource, query pluginmodels.DataQuery) (
	pluginmodels.DataResponse, error) {
	plugin := s.PluginManager.GetDataPlugin(ds.Type)
	if plugin == nil {
		factory, exists := s.registry[ds.Type]
		if !exists {
			return pluginmodels.DataResponse{}, fmt.Errorf(
				"could not find plugin corresponding to data source type: %q", ds.Type)
		}

		endpoint, err := factory(ds)
		if err != nil {
			return pluginmodels.DataResponse{}, fmt.Errorf("could not instantiate endpoint for data plugin %q: %w",
				ds.Type, err)
		}
		return endpoint.DataQuery(ctx, ds, query)
	}

	return plugin.DataQuery(ctx, ds, query)
}

// RegisterQueryHandler registers a query handler factory.
// This is only exposed for tests!
func (s *Service) RegisterQueryHandler(name string, factory func(*models.DataSource) (pluginmodels.DataPlugin, error)) {
	s.registry[name] = factory
}