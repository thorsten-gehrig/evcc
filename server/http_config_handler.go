package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger"
	"github.com/evcc-io/evcc/meter"
	"github.com/evcc-io/evcc/util/config"
	"github.com/evcc-io/evcc/util/templates"
	"github.com/evcc-io/evcc/vehicle"
	"github.com/gorilla/mux"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// templatesHandler returns the list of templates by class
func templatesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	class, err := templates.ClassString(vars["class"])
	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	res := templates.ByClass(class)

	lang := r.URL.Query().Get("lang")
	templates.EncoderLanguage(lang)

	if name := r.URL.Query().Get("name"); name != "" {
		for _, t := range res {
			if t.TemplateDefinition.Template == name {
				jsonResult(w, t)
				return
			}
		}

		jsonError(w, http.StatusBadRequest, errors.New("template not found"))
		return
	}

	jsonResult(w, res)
}

type product struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type products []product

func (p products) MarshalJSON() (out []byte, err error) {
	if p == nil {
		return []byte(`null`), nil
	}
	if len(p) == 0 {
		return []byte(`{}`), nil
	}

	out = append(out, '{')
	for _, e := range p {
		key, err := json.Marshal(e.Name)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(e.Template)
		if err != nil {
			return nil, err
		}
		out = append(out, key...)
		out = append(out, ':')
		out = append(out, val...)
		out = append(out, ',')
	}

	// replace last ',' with '}'
	if len(out) > 1 {
		out[len(out)-1] = '}'
	} else {
		out = append(out, '}')
	}

	return out, nil
}

// productsHandler returns the list of products by class
func productsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	class, err := templates.ClassString(vars["class"])
	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	tmpl := templates.ByClass(class)
	lang := r.URL.Query().Get("lang")

	res := make(products, 0)
	for _, t := range tmpl {
		for _, p := range t.Products {
			res = append(res, product{
				Name:     p.Title(lang),
				Template: t.TemplateDefinition.Template,
			})
		}
	}

	slices.SortFunc(res, func(a, b product) bool {
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	jsonResult(w, res)
}

// devicesHandler tests a configuration by class
func devicesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	class, err := templates.ClassString(vars["class"])
	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	var named []config.Named

	switch class {
	case templates.Meter:
		named = config.MetersConfig()
	case templates.Charger:
		named = config.ChargersConfig()
	case templates.Vehicle:
		named = config.VehiclesConfig()
	}

	res := make([]map[string]any, 0, len(named))
	for _, v := range named {
		conf := maps.Clone(v.Other)
		conf["type"] = v.Type

		res = append(res, conf)
	}

	jsonResult(w, res)
}

// newDeviceHandler creates a new device by class
func newDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	class, err := templates.ClassString(vars["class"])
	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	named := config.Named{
		Type:  "template",
		Other: req,
	}
	if req["type"] != nil {
		named.Type = req["type"].(string)
		delete(req, "type")
	}
	if req["Name"] != nil {
		named.Name = req["name"].(string)
		delete(req, "name")
	}

	var dev any
	var id int

	switch class {
	case templates.Charger:
		var c api.Charger
		if c, err = charger.NewFromConfig(named.Type, req); err == nil {
			if err = config.AddCharger(named, c); err == nil {
				id, _ = config.ChargerID(named.Name)
				dev = c
			}
		}
	case templates.Meter:
		var m api.Meter
		if m, err = meter.NewFromConfig(named.Type, req); err == nil {
			if err = config.AddMeter(named, m); err == nil {
				id, _ = config.MeterID(named.Name)
				dev = m
			}
		}
	case templates.Vehicle:
		var v api.Vehicle
		if v, err = vehicle.NewFromConfig(named.Type, req); err == nil {
			if err = config.AddVehicle(named, v); err == nil {
				id, _ = config.VehicleID(named.Name)
				dev = v
			}
		}
	}

	_ = dev

	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	res := struct {
		ID int `json:"id"`
	}{
		ID: id,
	}

	jsonResult(w, res)
}

// testHandler tests a configuration by class
func testHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	class, err := templates.ClassString(vars["class"])
	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	typ := "template"
	if req["type"] != nil {
		typ = req["type"].(string)
		delete(req, "type")
	}

	var dev any

	switch class {
	case templates.Charger:
		dev, err = charger.NewFromConfig(typ, req)
	case templates.Meter:
		dev, err = meter.NewFromConfig(typ, req)
	case templates.Vehicle:
		dev, err = vehicle.NewFromConfig(typ, req)
	}

	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}

	type result = struct {
		Value any   `json:"value"`
		Error error `json:"error"`
	}

	res := make(map[string]result)

	if dev, ok := dev.(api.Meter); ok {
		val, err := dev.CurrentPower()
		res["CurrentPower"] = result{val, err}
	}

	if dev, ok := dev.(api.MeterEnergy); ok {
		val, err := dev.TotalEnergy()
		res["TotalEnergy"] = result{val, err}
	}

	if dev, ok := dev.(api.Battery); ok {
		val, err := dev.Soc()
		res["Soc"] = result{val, err}
	}

	jsonResult(w, res)
}
