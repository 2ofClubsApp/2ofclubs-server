package app

import (
	"context"
	"fmt"
	"github.com/2-of-clubs/2ofclubs-server/app/handler"
	"github.com/2-of-clubs/2ofclubs-server/app/logger"
	"github.com/2-of-clubs/2ofclubs-server/app/model"
	"github.com/2-of-clubs/2ofclubs-server/app/status"
	"github.com/2-of-clubs/2ofclubs-server/config"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"log"
	"net/http"
)

type routeHandler func(w http.ResponseWriter, r *http.Request)
type hdlr func(db *gorm.DB, redis *redis.Client, w http.ResponseWriter, r *http.Request, s *status.Status) (httpStatus int, err error)

// App - API config for DB, Mux Router and CORS
type App struct {
	db      *gorm.DB
	redis   *redis.Client
	router  *mux.Router
	origin  handlers.CORSOption
	methods handlers.CORSOption
	headers handlers.CORSOption
}

// Initialize - Server initialization
// Database, CORS and the admin settings are initialized
func (app *App) Initialize(dbConfig *config.DBConfig, redisConfig *config.RedisConfig, adminConfig *model.User) {
	ctx := context.Background()
	dbFormat :=
		fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbConfig.Host,
			dbConfig.Port,
			dbConfig.User,
			dbConfig.Password,
			dbConfig.Name,
		)
	redisClient := redis.NewClient(
		&redis.Options{
			Addr:     redisConfig.Addr,
			Password: redisConfig.Password,
			DB:       redisConfig.DB,
		})
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Unable to connect to Redis\n", err)
	}
	db, err := gorm.Open(postgres.Open(dbFormat), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		//DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		log.Fatal("Unable to connect to database\n", err)
	}
	app.db = db
	app.redis = redisClient
	app.router = mux.NewRouter()
	//StrictSlash(true)
	app.router.Use(logger.LoggingMiddleware)
	// Note: Set this as env var later
	app.origin = handlers.AllowedOrigins([]string{"http://localhost:3000"})
	app.methods = handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions})
	app.headers = handlers.AllowedHeaders([]string{"Content-Type", "Authorization"})

	app.setRoutes()
	log.Println("Connected to Redis")
	log.Println("Connected to Database")
	if db.Migrator().CreateTable(model.NewEvent(), model.NewTag(), model.NewUserClub(), model.NewEvent()) != nil {
		log.Println("Base tables not created")
	}
	if db.AutoMigrate(model.NewUser(), model.NewClub()) != nil {
		log.Println("User and Club join tables not created")
	}
	if db.SetupJoinTable(&model.User{}, "Manages", &model.UserClub{}) != nil {
		log.Println("User Club join table not created")
	}

	// GORM already ensures the uniqueness of the username and email, thus we don't need to check if the admin already exists or not
	if db.Create(adminConfig).Error != nil {
		log.Println("Unable to create admin account. Account already exists")
	}

}

// Set all routes for API server
func (app *App) setRoutes() {
	// Signup Route
	app.Post("/signup", app.Handle(handler.SignUp, false)) // Done
	// Logout Route
	app.Post("/logout/{username}", app.Handle(handler.Logout, false)) // Done
	// Login Routes
	app.Post("/login", app.Handle(handler.Login, false)) // Done (Need to check for synchronous token (CSRF prevention))

	// Admin Route
	app.Post("/toggle/users/{username}", app.Handle(handler.ToggleUser, true))   // Done
	app.Post("/toggle/clubs/{cid:[0-9]+}", app.Handle(handler.ToggleClub, true)) // Done
	app.Get("/users/toggle", app.Handle(handler.GetToggleUser, true))            // Done
	app.Get("/clubs/toggle", app.Handle(handler.GetToggleClub, true))            // Done

	// User Routes
	app.Get("/users/{username}", app.Handle(handler.GetUser, true))                             // Done
	app.Post("/users/{username}/tags", app.Handle(handler.UpdateUserTags, true))                // Done
	app.Get("/users/{username}/manages", app.Handle(handler.GetUserClubsManage, true))          // Done
	app.Get("/users/{username}/attends", app.Handle(handler.GetUserEventsAttend, true))         // Done
	app.Post("/events/{eid:[0-9]+}/attend", app.Handle(handler.AddUserAttendsEvent, true))      // Done
	app.Post("/events/{eid:[0-9]+}/unattend", app.Handle(handler.RemoveUserAttendsEvent, true)) // Done
	app.Post("/resetpassword/{username}", app.Handle(handler.RequestResetUserPassword, false))  // Done
	app.Post("/resetpassword/{username}/{token}", app.Handle(handler.ResetUserPassword, false)) // Done
	app.Post("/changePassword/users/{username}", app.Handle(handler.UpdateUserPassword, true))  // Done

	// Tag Routes
	app.Get("/tags", app.Handle(handler.GetTags, false))               // Done
	app.Get("/tags/active", app.Handle(handler.GetActiveTags, false))  // Done
	app.Post("/tags", app.Handle(handler.CreateTag, true))             // Done
	app.Post("/upload/tags", app.Handle(handler.UploadTagsList, true)) // Done
	app.Post("/upload/photos/clubs/{cid:[0-9]+}", app.Handle(handler.UploadClubPhoto, true))
	app.Get("/photos/clubs/{cid:[0-9]+}", app.Handle(handler.GetClubPhoto, false))
	app.Post("/toggle/tags/{tagName}", app.Handle(handler.ToggleTag, true)) // Done

	// Club routes
	app.Post("/clubs", app.Handle(handler.CreateClub, true)) // Done
	//app.Post("/clubs/{cid:[0-9]+}", app.Handle(handler.UpdateClub, true)) // Requires Fixing
	app.Get("/clubs/{cid:[0-9]+}", app.Handle(handler.GetClub, false)) // Done

	app.Post("/clubs/{cid:[0-9]+}/manages/{username}", app.Handle(handler.AddManager, true))      // Done (Adding managers/maintainers to club)
	app.Delete("/clubs/{cid:[0-9]+}/manages/{username}", app.Handle(handler.RemoveManager, true)) // Partially done (Removing managers/maintainers) (If the current owner wants to leave, then they must appoint a new person)
	app.Post("/clubs/{cid:[0-9]+}/tags", app.Handle(handler.UpdateClubTags, true))                // Done
	app.Get("/clubs", app.Handle(handler.GetClubs, false))                                        // In-Progress

	app.Get("/events", app.Handle(handler.GetAllEvents, false))                       // Done
	app.Get("/events/{eid:[0-9]+}", app.Handle(handler.GetEvent, false))              // Done
	app.Get("/clubs/{cid:[0-9]+}/events", app.Handle(handler.GetClubEvents, false))   // Done
	app.Post("/clubs/{cid:[0-9]+}/events", app.Handle(handler.CreateClubEvent, true)) // Done

	//app.Post("/clubs/{cid:[0-9]+}/events/{eid:[0-9]+}", app.Handle(handler.UpdateClubEvent, true))   // Done
	app.Delete("/clubs/{cid:[0-9]+}/events/{eid:[0-9]+}", app.Handle(handler.DeleteClubEvent, true)) // Done

	// 404 Route
	app.router.NotFoundHandler = notFound() // Done
}

// Run - Main run function to startup API serve
func (app *App) Run(port string) {
	err := http.ListenAndServe(port, handlers.CORS(app.origin, app.methods, app.headers)(app.router))
	if err != nil {
		panic(err)
	}
}

// Post - Setting a POST route and its associated handler
func (app *App) Post(path string, f routeHandler) {
	app.router.HandleFunc(path, f).Methods(http.MethodPost)
}

// Get - Setting a GET route and its associated handler
func (app *App) Get(path string, f routeHandler) {
	app.router.HandleFunc(path, f).Methods(http.MethodGet)
}

// Delete - Setting a Delete route and its associated handler
func (app *App) Delete(path string, f routeHandler) {
	app.router.HandleFunc(path, f).Methods(http.MethodDelete)
}

// Handle - Wrapper function to return a base Handler function
func (app *App) Handle(h hdlr, verifyRequest bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Must verify for sensitive information
		s := status.New()
		if verifyRequest {
			if isValid := handler.VerifyJWT(r); isValid && handler.IsActiveToken(app.redis, r) {
				if httpStatus, err := h(app.db, app.redis, w, r, s); err != nil {
					WriteData(http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, w)
				} else {
					WriteData(s.Display(), httpStatus, w)
				}
			} else {
				WriteData(http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized, w)
			}
		} else {
			if httpStatus, err := h(app.db, app.redis, w, r, s); err != nil {
				WriteData(http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, w)
			} else {
				WriteData(s.Display(), httpStatus, w)
			}
		}
	}
}

// 404 Not Found handler for mismatched routes
func notFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteData(http.StatusText(http.StatusNotFound), http.StatusNotFound, w)
	})
}

// WriteData - Return response message and an HTTP Status Code upon receiving a request.
func WriteData(data string, code int, w http.ResponseWriter) int {
	w.WriteHeader(code)
	n, err := fmt.Fprint(w, data)
	if err != nil {
		return -1
	}
	return n
}
