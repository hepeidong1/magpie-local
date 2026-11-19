package gateway

// Group-only mode (settings.GroupsOnly): a port that serves routing groups
// alone. Every request's model is resolved to a group — a name it already
// carries, a rewrite of the name an agent asks by, or the port's Default —
// and a name that resolves to none is refused, so /v1/models can list the
// groups alone and an agent of the port can ask for a group and nothing
// else. The group's own routing (members in order, failover, rests) is the
// gateway's, unchanged behind the port.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// ErrOnlyGroups is what a request gets for a model that is no group on a
// group-only port.
var ErrOnlyGroups = errors.New("only group names are allowed on this port")

// groupOnlyConfig is the port's group-only shape: the one a test or a
// caller set, else the port's setting (settings.GroupsOnly), read per
// request as the settings are (Shared).
func (s *Server) groupOnlyConfig() *settings.GroupsOnly {
	if s != nil && s.groupOnly != nil {
		return s.groupOnly
	}
	return settings.Shared().GroupsOnly
}

// groupOnlyEnabled is whether the port is group-only. 本分叉：恒真——端口
// 只接受路由组的暴露名（裸组名），其余一律拒绝。
func (s *Server) groupOnlyEnabled() bool {
	return true
}

// servedGroupRef is ref as the group it names (its id), where that group
// is served here. 本分叉：只认裸组名（组 id 或组 Name，大小写不敏感）——
// provider.GroupFor 本身就是裸名解析，含 "/" 的（provider/model、
// group/<id>）一律逐不到组，返回 false。
func servedGroupRef(ref string) (string, bool) {
	if !provider.GroupsServed() {
		return "", false
	}
	gid, ok := provider.GroupFor(ref)
	if !ok {
		return "", false
	}
	return strings.TrimPrefix(gid, provider.GroupPrefix), true
}

// groupOnlyAllows says group id may be served on this port: Allowed, when
// set, is the whole of it, and a group not in it is served as little as it
// is listed.
func (s *Server) groupOnlyAllows(id string) bool {
	c := s.groupOnlyConfig()
	if c == nil || len(c.Allowed) == 0 {
		return true
	}
	for _, a := range c.Allowed {
		if aid, ok := servedGroupRef(a); ok && aid == id {
			return true
		}
	}
	return false
}

// groupOnlyServeable is ref as the id of the group the port may serve by
// it, or false.
func (s *Server) groupOnlyServeable(ref string) (string, bool) {
	id, ok := servedGroupRef(ref)
	if !ok || !s.groupOnlyAllows(id) {
		return "", false
	}
	return id, true
}

// resolveGroupOnlyModel is m as the group a group-only port serves it by,
// or ErrOnlyGroups: the name itself, its Rewrite, else Default. A bare
// group name is returned as "group/<id>", the id the gateway routes by.
func (s *Server) resolveGroupOnlyModel(m string) (string, error) {
	if !s.groupOnlyEnabled() {
		return m, nil
	}
	c := s.groupOnlyConfig()
	if c == nil {
		c = &settings.GroupsOnly{}
	}
	m = strings.TrimSpace(m)
	if m != "" {
		if g, ok := c.Rewrite[m]; ok {
			if _, ok := s.groupOnlyServeable(g); !ok {
				return "", ErrOnlyGroups
			}
			return provider.GroupPrefix + g, nil
		}
		if _, ok := s.groupOnlyServeable(m); ok {
			return provider.GroupPrefix + m, nil
		}
	}
	if c.Default != "" {
		if _, ok := s.groupOnlyServeable(c.Default); ok {
			return provider.GroupPrefix + c.Default, nil
		}
	}
	return "", ErrOnlyGroups
}

func writeGroupOnlyError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if errors.Is(err, ErrOnlyGroups) {
		msg = ErrOnlyGroups.Error()
	}
	// 本分叉：非组名一律 404
	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": msg, "type": "invalid_request_error"}})
}

// setRequestModel puts model in the request body as its model field, so a
// rewritten (auto's, Default's) request goes up under the group's name.
func setRequestModel(body []byte, model string) ([]byte, error) {
	return withModel(body, model), nil
}

// groupOnlyModels answers /v1/models with the routing groups alone, each
// by the bare name the port accepts (its id, or its Name): the members of
// a group are never listed, so an agent can ask for a group and nothing
// else.
func (s *Server) groupOnlyModels(w http.ResponseWriter, r *http.Request) {
	shown := catalogFor(r)
	labels := provider.Labels(shown)
	data := []map[string]any{}
	for i, e := range shown {
		if e.Group == "" {
			continue
		}
		if _, ok := s.groupOnlyServeable(e.Group); !ok {
			continue
		}
		m := modelObject(e)
		m["id"] = e.Group // the bare group id: what an agent asks the port by
		m["magpie_label"] = labels[i]
		data = append(data, m)
	}
	if r.URL.Query().Get("format") == "text" {
		var b strings.Builder
		for _, m := range data {
			fmt.Fprintln(&b, m["id"])
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, b.String())
		return
	}
	out := map[string]any{"object": "list", "data": data, "has_more": false}
	if len(data) > 0 {
		out["first_id"], out["last_id"] = data[0]["id"], data[len(data)-1]["id"]
	}
	writeJSON(w, 200, out)
}
