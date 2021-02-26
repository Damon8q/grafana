package manager

import (
	"os"
	"path/filepath"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	pluginmodels "github.com/grafana/grafana/pkg/plugins/models"
)

type PluginDashboardInfoDTO struct {
	PluginId         string `json:"pluginId"`
	Title            string `json:"title"`
	Imported         bool   `json:"imported"`
	ImportedUri      string `json:"importedUri"`
	ImportedUrl      string `json:"importedUrl"`
	Slug             string `json:"slug"`
	DashboardId      int64  `json:"dashboardId"`
	FolderId         int64  `json:"folderId"`
	ImportedRevision int64  `json:"importedRevision"`
	Revision         int64  `json:"revision"`
	Description      string `json:"description"`
	Path             string `json:"path"`
	Removed          bool   `json:"removed"`
}

func (pm *PluginManager) GetPluginDashboards(orgId int64, pluginId string) ([]*PluginDashboardInfoDTO, error) {
	plugin, exists := pm.Plugins[pluginId]
	if !exists {
		return nil, pluginmodels.PluginNotFoundError{pluginId}
	}

	result := make([]*PluginDashboardInfoDTO, 0)

	// load current dashboards
	query := models.GetDashboardsByPluginIdQuery{OrgId: orgId, PluginId: pluginId}
	if err := bus.Dispatch(&query); err != nil {
		return nil, err
	}

	existingMatches := make(map[int64]bool)
	for _, include := range plugin.Includes {
		if include.Type != pluginmodels.PluginTypeDashboard {
			continue
		}

		dashboard, err := LoadPluginDashboard(plugin.Id, include.Path)
		if err != nil {
			return nil, err
		}

		res := &PluginDashboardInfoDTO{}
		res.Path = include.Path
		res.PluginId = plugin.Id
		res.Title = dashboard.Title
		res.Revision = dashboard.Data.Get("revision").MustInt64(1)

		// find existing dashboard
		for _, existingDash := range query.Result {
			if existingDash.Slug == dashboard.Slug {
				res.DashboardId = existingDash.Id
				res.Imported = true
				res.ImportedUri = "db/" + existingDash.Slug
				res.ImportedUrl = existingDash.GetUrl()
				res.ImportedRevision = existingDash.Data.Get("revision").MustInt64(1)
				existingMatches[existingDash.Id] = true
			}
		}

		result = append(result, res)
	}

	// find deleted dashboards
	for _, dash := range query.Result {
		if _, exists := existingMatches[dash.Id]; !exists {
			result = append(result, &PluginDashboardInfoDTO{
				Slug:        dash.Slug,
				DashboardId: dash.Id,
				Removed:     true,
			})
		}
	}

	return result, nil
}

func (pm *PluginManager) loadPluginDashboard(pluginId, path string) (*models.Dashboard, error) {
	plugin, exists := pm.Plugins[pluginId]
	if !exists {
		return nil, pluginmodels.PluginNotFoundError{pluginId}
	}

	dashboardFilePath := filepath.Join(plugin.PluginDir, path)
	// nolint:gosec
	// We can ignore the gosec G304 warning on this one because `plugin.PluginDir` is based
	// on plugin folder structure on disk and not user input. `path` comes from the
	// `plugin.json` configuration file for the loaded plugin
	reader, err := os.Open(dashboardFilePath)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := reader.Close(); err != nil {
			pm.log.Warn("Failed to close file", "path", dashboardFilePath, "err", err)
		}
	}()

	data, err := simplejson.NewFromReader(reader)
	if err != nil {
		return nil, err
	}

	return models.NewDashboardFromJson(data), nil
}