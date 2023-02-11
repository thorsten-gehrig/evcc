package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger"
	"github.com/evcc-io/evcc/meter"
	"github.com/evcc-io/evcc/provider/mqtt"
	"github.com/evcc-io/evcc/push"
	"github.com/evcc-io/evcc/server"
	"github.com/evcc-io/evcc/server/oauth2redirect"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/config"
	"github.com/evcc-io/evcc/util/modbus"
	"github.com/evcc-io/evcc/vehicle"
	"github.com/evcc-io/evcc/vehicle/wrapper"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

var conf = globalConfig{
	Interval: 10 * time.Second,
	Log:      "info",
	Network: networkConfig{
		Schema: "http",
		Host:   "evcc.local",
		Port:   7070,
	},
	Mqtt: mqttConfig{
		Topic: "evcc",
	},
	Database: dbConfig{
		Type: "sqlite",
		Dsn:  "~/.evcc/evcc.db",
	},
}

type globalConfig struct {
	URI          interface{} // TODO deprecated
	Network      networkConfig
	Log          string
	SponsorToken string
	Plant        string // telemetry plant id
	Telemetry    bool
	Metrics      bool
	Profile      bool
	Levels       map[string]string
	Interval     time.Duration
	Database     dbConfig
	Mqtt         mqttConfig
	ModbusProxy  []proxyConfig
	Javascript   []javascriptConfig
	Influx       server.InfluxConfig
	EEBus        map[string]interface{}
	HEMS         config.Typed
	Messaging    messagingConfig
	Meters       []config.Named
	Chargers     []config.Named
	Vehicles     []config.Named
	Tariffs      tariffConfig
	Site         map[string]interface{}
	Loadpoints   []map[string]interface{}
}

type mqttConfig struct {
	mqtt.Config `mapstructure:",squash"`
	Topic       string
}

type javascriptConfig struct {
	VM     string
	Script string
}

type proxyConfig struct {
	Port            int
	ReadOnly        bool
	modbus.Settings `mapstructure:",squash"`
}

type dbConfig struct {
	Type string
	Dsn  string
}

type messagingConfig struct {
	Events   map[string]push.EventTemplateConfig
	Services []config.Typed
}

type tariffConfig struct {
	Currency string
	Grid     config.Typed
	FeedIn   config.Typed
	Planner  config.Typed
}

type networkConfig struct {
	Schema string
	Host   string
	Port   int
}

func (c networkConfig) HostPort() string {
	if c.Schema == "http" && c.Port == 80 || c.Schema == "https" && c.Port == 443 {
		return c.Host
	}
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c networkConfig) URI() string {
	return fmt.Sprintf("%s://%s", c.Schema, c.HostPort())
}

// configureAuth handles routing for devices. For now only api.AuthProvider related routes
func configureAuth(conf networkConfig, vehicles []api.Vehicle, router *mux.Router, paramC chan<- util.Param) {
	auth := router.PathPrefix("/oauth").Subrouter()
	auth.Use(handlers.CompressHandler)
	auth.Use(handlers.CORS(
		handlers.AllowedHeaders([]string{"Content-Type"}),
	))

	// wire the handler
	oauth2redirect.SetupRouter(auth)

	// initialize
	authCollection := util.NewAuthCollection(paramC)

	baseURI := conf.URI()
	baseAuthURI := fmt.Sprintf("%s/oauth", baseURI)

	var id int
	for _, v := range vehicles {
		if provider, ok := v.(api.AuthProvider); ok {
			id += 1

			basePath := fmt.Sprintf("vehicles/%d", id)
			callbackURI := fmt.Sprintf("%s/%s/callback", baseAuthURI, basePath)

			// register vehicle
			ap := authCollection.Register(fmt.Sprintf("oauth/%s", basePath), v.Title())

			provider.SetCallbackParams(baseURI, callbackURI, ap.Handler())

			auth.
				Methods(http.MethodPost).
				Path(fmt.Sprintf("/%s/login", basePath)).
				HandlerFunc(provider.LoginHandler())
			auth.
				Methods(http.MethodPost).
				Path(fmt.Sprintf("/%s/logout", basePath)).
				HandlerFunc(provider.LogoutHandler())

			log.INFO.Printf("ensure the oauth client redirect/callback is configured for %s: %s", v.Title(), callbackURI)
		}
	}

	authCollection.Publish()
}

func configureMeters(conf []config.Named) error {
	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create meter %d: missing name", i+1)
		}

		m, err := meter.NewFromConfig(cc.Type, cc.Other)
		if err != nil {
			err = fmt.Errorf("cannot create meter '%s': %w", cc.Name, err)
			return err
		}

		if err := config.AddMeter(cc, m); err != nil {
			return err
		}
	}

	return nil
}

func configureChargers(conf []config.Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create charger %d: missing name", i+1)
		}

		cc := cc

		g.Go(func() error {
			c, err := charger.NewFromConfig(cc.Type, cc.Other)
			if err != nil {
				return fmt.Errorf("cannot create charger '%s': %w", cc.Name, err)
			}

			mu.Lock()
			defer mu.Unlock()

			return config.AddCharger(cc, c)
		})
	}

	return g.Wait()
}

func configureVehicles(conf []config.Named) error {
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())

	for i, cc := range conf {
		if cc.Name == "" {
			return fmt.Errorf("cannot create vehicle %d: missing name", i+1)
		}

		cc := cc

		g.Go(func() error {
			v, err := vehicle.NewFromConfig(cc.Type, cc.Other)
			if err != nil {
				var ce *util.ConfigError
				if errors.As(err, &ce) {
					return fmt.Errorf("cannot create vehicle '%s': %w", cc.Name, err)
				}

				// wrap non-config vehicle errors to prevent fatals
				v = wrapper.New(err)
			}

			// ensure vehicle config has title
			if v.Title() == "" {
				//lint:ignore SA1019 as Title is safe on ascii
				v.SetTitle(strings.Title(cc.Name))
			}

			mu.Lock()
			defer mu.Unlock()

			return config.AddVehicle(cc, v)
		})
	}

	return g.Wait()
}
