package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/paroxity/portal"
	"github.com/paroxity/portal/internal"
	portallog "github.com/paroxity/portal/log"
	portalserver "github.com/paroxity/portal/server"
	"github.com/paroxity/portal/session"
	"github.com/paroxity/portal/socket"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "15:04:05",
	})
	conf := readConfig(logger)
	if conf.Logger.File != "" {
		fileLogger, err := portallog.New(conf.Logger.File)
		if err != nil {
			logger.Fatalf("unable to create file logger: %v", err)
		}
		logger.SetOutput(fileLogger)
	}
	level, err := logrus.ParseLevel(conf.Logger.Level)
	if err != nil {
		logger.Errorf("unable to parse log level '%s': %v", conf.Logger.Level, err)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	resourcePacks, err := portal.LoadResourcePacks(conf.ResourcePacks.Directory)
	if err != nil {
		logger.Fatalf("unable to load resource packs: %v", err)
	}
	for i, pack := range resourcePacks {
		key, ok := conf.ResourcePacks.EncryptionKeys[pack.UUID()]
		if ok {
			resourcePacks[i] = pack.WithContentKey(key)
		}
	}

	p := portal.New(portal.Options{
		Logger: logger,

		Address: conf.Network.Address,
		ListenConfig: minecraft.ListenConfig{
			StatusProvider: portal.NewMOTDStatusProvider("Portal"),

			ResourcePacks:        resourcePacks,
			TexturePacksRequired: conf.ResourcePacks.Required,
		},

		Whitelist: session.NewSimpleWhitelist(conf.Whitelist.Enabled, conf.Whitelist.Players),
	})
	if err := p.Listen(); err != nil {
		logger.Fatalf("failed to listen on %s: %v", conf.Network.Address, err)
	}

	socketServer := socket.NewDefaultServer(conf.Network.Communication.Address, conf.Network.Communication.Secret, p.SessionStore(), p.ServerRegistry(), logger)
	if err := socketServer.Listen(); err != nil {
		p.Logger().Fatalf("socket server failed to listen: %v", err)
	}
	if conf.PlayerLatency.Report {
		go socketServer.ReportPlayerLatency(time.Second * time.Duration(conf.PlayerLatency.UpdateInterval))
	}

	logger.Info("starting dummy Dragonfly server for connection fallback...")
	uConf := server.DefaultConfig()
	uConf.Network.Address = ":19131"
	uConf.Server.AuthEnabled = false
	uConf.Players.SaveData = false
	uConf.World.SaveData = false
	uConf.Players.MaximumChunkRadius = 0

	srvConf, _ := uConf.Config(logger)
	srvConf.Generator = func(_ world.Dimension) world.Generator { return world.NopGenerator{} }
	srv := srvConf.New()
	srv.World().StopWeatherCycle()
	srv.CloseOnProgramEnd()
	srv.Listen()
	go func() {
		for srv.Accept(func(p *player.Player) {
			p.SetGameMode(world.GameModeSpectator)
		}) {
		}
	}()

	session.Srv = portalserver.New("Hibernation", ":19131")
	// go func() {
	// 	reader := bufio.NewReader(os.Stdin)
	// 	reader.ReadRune()
	// 	logger.Info("registered fallback server")
	// 	p.ServerRegistry().AddServer(session.Srv)
	// }()

	for {
		s, err := p.Accept()
		if err != nil {
			s.Disconnect(text.Colourf("<red>%v</red>", err))
			p.Logger().Errorf("failed to accept connection: %v", err)
			continue
		}
		_ = s
	}
}

func readConfig(logger internal.Logger) portal.Config {
	c := portal.DefaultConfig()
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		f, err := os.Create("config.json")
		if err != nil {
			logger.Fatalf("error creating config: %v", err)
		}
		data, err := json.MarshalIndent(c, "", "\t")
		if err != nil {
			logger.Fatalf("error encoding default config: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			logger.Fatalf("error writing encoded default config: %v", err)
		}
		_ = f.Close()
	}
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		logger.Fatalf("error reading config: %v", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		logger.Fatalf("error decoding config: %v", err)
	}
	return c
}
