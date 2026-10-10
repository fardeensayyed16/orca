package gui

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/types"
)

type pageData struct {
	Nodes          []*types.Node
	Services       []*types.Service
	Replicas       []*types.Replica
	Page           string
	DaemonStatus   string
	ServiceCount   int
	RunningCount   int
	HealthyCount   int
	ReadyCount     int
	NodeReadyCount int
}

func RegisterRoutes(mux *http.ServeMux, store storage.Storage, _ string) {
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, store, "index")
	})
	mux.HandleFunc("GET /nodes", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, store, "nodes")
	})
	mux.HandleFunc("GET /services", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, store, "services")
	})
	mux.HandleFunc("GET /pods", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, store, "pods")
	})
	mux.HandleFunc("GET /api/v1/gui/summary", func(w http.ResponseWriter, r *http.Request) {
		if !allowGET(w, r) {
			return
		}
		data, ok := loadPageData(w, r, store)
		if ok {
			render(w, "summary", data)
		}
	})
	mux.HandleFunc("GET /api/v1/gui/nodes", func(w http.ResponseWriter, r *http.Request) {
		if !allowGET(w, r) {
			return
		}
		data, ok := loadPageData(w, r, store)
		if ok {
			render(w, "nodes_table", data)
		}
	})
	mux.HandleFunc("GET /api/v1/gui/services", func(w http.ResponseWriter, r *http.Request) {
		if !allowGET(w, r) {
			return
		}
		data, ok := loadPageData(w, r, store)
		if ok {
			render(w, "services_table", data)
		}
	})
	mux.HandleFunc("GET /api/v1/gui/pods", func(w http.ResponseWriter, r *http.Request) {
		if !allowGET(w, r) {
			return
		}
		data, ok := loadPageData(w, r, store)
		if ok {
			render(w, "pods_table", data)
		}
	})
	mux.HandleFunc("POST /api/v1/gui/deploy", func(w http.ResponseWriter, r *http.Request) {
		if !parseForm(w, r) {
			return
		}
		replicas, err := strconv.Atoi(r.FormValue("replicas"))
		if err != nil || replicas < 1 || strings.TrimSpace(r.FormValue("name")) == "" ||
			strings.TrimSpace(r.FormValue("image")) == "" {
			http.Error(w, "name, image, and a positive replica count are required", http.StatusBadRequest)
			return
		}
		err = store.DeployService(r.Context(), &types.Service{
			Name: strings.TrimSpace(r.FormValue("name")), Image: strings.TrimSpace(r.FormValue("image")),
			Replicas: replicas,
		})
		if err != nil {
			http.Error(w, "deploy service: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/services", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /api/v1/gui/scale", func(w http.ResponseWriter, r *http.Request) {
		if !parseForm(w, r) {
			return
		}
		replicas, err := strconv.Atoi(r.FormValue("replicas"))
		if err != nil || replicas < 1 || strings.TrimSpace(r.FormValue("name")) == "" {
			http.Error(w, "service name and a positive replica count are required", http.StatusBadRequest)
			return
		}
		if err := store.UpdateServiceReplicas(r.Context(), strings.TrimSpace(r.FormValue("name")), replicas); err != nil {
			http.Error(w, "scale service: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/services", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /api/v1/gui/delete-service", func(w http.ResponseWriter, r *http.Request) {
		if !parseForm(w, r) {
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "service name is required", http.StatusBadRequest)
			return
		}
		if err := store.DeleteService(r.Context(), name); err != nil {
			http.Error(w, "delete service: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/services", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /api/v1/gui/delete-node", func(w http.ResponseWriter, r *http.Request) {
		if !parseForm(w, r) {
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "node name is required", http.StatusBadRequest)
			return
		}
		if err := store.DeleteNode(r.Context(), name); err != nil {
			http.Error(w, "delete node: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/nodes", http.StatusSeeOther)
	})
}

func renderPage(w http.ResponseWriter, r *http.Request, store storage.Storage, name string) {
	if name == "index" && r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !allowGET(w, r) {
		return
	}
	data, ok := loadPageData(w, r, store)
	if ok {
		data.Page = name
		render(w, name, data)
	}
}

func allowGET(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !loopbackHost(r.Host) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || !loopbackHost(parsed.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}
	}
	return true
}

func loopbackHost(host string) bool {
	return host == "127.0.0.1:9000" || host == "localhost:9000"
}

func loadPageData(w http.ResponseWriter, r *http.Request, store storage.Storage) (pageData, bool) {
	nodes, err := store.ListNodes(r.Context())
	if err != nil {
		http.Error(w, "list nodes: "+err.Error(), http.StatusInternalServerError)
		return pageData{}, false
	}
	services, err := store.ListServices(r.Context())
	if err != nil {
		http.Error(w, "list services: "+err.Error(), http.StatusInternalServerError)
		return pageData{}, false
	}
	replicas, err := store.ListReplicas(r.Context())
	if err != nil {
		http.Error(w, "list replicas: "+err.Error(), http.StatusInternalServerError)
		return pageData{}, false
	}
	data := pageData{
		Nodes: nodes, Services: services, Replicas: replicas, DaemonStatus: "Running",
		ServiceCount: len(services),
	}
	for _, node := range nodes {
		if node.Status == "Ready" {
			data.NodeReadyCount++
		}
	}
	for _, replica := range replicas {
		if replica.Status == types.PodStatusRunning {
			data.RunningCount++
		}
		if replica.Health == types.HealthStatusHealthy {
			data.HealthyCount++
		}
		if replica.Ready {
			data.ReadyCount++
		}
	}
	return data, true
}

func render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("render %s: %v", name, err), http.StatusInternalServerError)
	}
}

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}
