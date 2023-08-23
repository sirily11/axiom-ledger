package loggers

import (
	"github.com/sirupsen/logrus"

	"github.com/axiomesh/axiom-kit/log"
	"github.com/axiomesh/axiom/pkg/repo"
)

const (
	P2P      = "p2p"
	Order    = "order"
	Executor = "executor"
	Router   = "router"
	App      = "app"
	API      = "api"
	CoreAPI  = "coreapi"
	Storage  = "storage"
	Profile  = "profile"
	Finance  = "finance"
)

var w = &LoggerWrapper{
	loggers: map[string]*logrus.Entry{
		P2P:      log.NewWithModule(P2P),
		Order:    log.NewWithModule(Order),
		Executor: log.NewWithModule(Executor),
		Router:   log.NewWithModule(Router),
		App:      log.NewWithModule(App),
		API:      log.NewWithModule(API),
		CoreAPI:  log.NewWithModule(CoreAPI),
		Storage:  log.NewWithModule(Storage),
		Profile:  log.NewWithModule(Profile),
		Finance:  log.NewWithModule(Finance),
	},
}

type LoggerWrapper struct {
	loggers map[string]*logrus.Entry
}

func Initialize(config *repo.Config) {
	m := make(map[string]*logrus.Entry)
	m[P2P] = log.NewWithModule(P2P)
	m[P2P].Logger.SetLevel(log.ParseLevel(config.Log.Module.P2P))
	m[Order] = log.NewWithModule(Order)
	m[Order].Logger.SetLevel(log.ParseLevel(config.Log.Module.Consensus))
	m[Executor] = log.NewWithModule(Executor)
	m[Executor].Logger.SetLevel(log.ParseLevel(config.Log.Module.Executor))
	m[Router] = log.NewWithModule(Router)
	m[Router].Logger.SetLevel(log.ParseLevel(config.Log.Module.Router))
	m[App] = log.NewWithModule(App)
	m[App].Logger.SetLevel(log.ParseLevel(config.Log.Level))
	m[API] = log.NewWithModule(API)
	m[API].Logger.SetLevel(log.ParseLevel(config.Log.Module.API))
	m[CoreAPI] = log.NewWithModule(CoreAPI)
	m[CoreAPI].Logger.SetLevel(log.ParseLevel(config.Log.Module.CoreAPI))
	m[Storage] = log.NewWithModule(Storage)
	m[Storage].Logger.SetLevel(log.ParseLevel(config.Log.Module.Storage))
	m[Profile] = log.NewWithModule(Profile)
	m[Profile].Logger.SetLevel(log.ParseLevel(config.Log.Module.Profile))
	m[Finance] = log.NewWithModule(Finance)
	m[Finance].Logger.SetLevel(log.ParseLevel(config.Log.Module.Finance))

	w = &LoggerWrapper{loggers: m}
}

func ReConfig(config *repo.Config) {
	m := w.loggers
	m[P2P].Logger.SetLevel(log.ParseLevel(config.Log.Module.P2P))
	m[Order].Logger.SetLevel(log.ParseLevel(config.Log.Module.Consensus))
	m[Executor].Logger.SetLevel(log.ParseLevel(config.Log.Module.Executor))
	m[Router].Logger.SetLevel(log.ParseLevel(config.Log.Module.Router))
	m[App].Logger.SetLevel(log.ParseLevel(config.Log.Level))
	m[API].Logger.SetLevel(log.ParseLevel(config.Log.Module.API))
	m[CoreAPI].Logger.SetLevel(log.ParseLevel(config.Log.Module.CoreAPI))
	m[Storage].Logger.SetLevel(log.ParseLevel(config.Log.Module.Storage))
	m[Profile].Logger.SetLevel(log.ParseLevel(config.Log.Module.Profile))
}

func Logger(name string) logrus.FieldLogger {
	return w.loggers[name]
}