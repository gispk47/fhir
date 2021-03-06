package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/itsjamie/gin-cors"
	"gopkg.in/mgo.v2"
)

type AfterRoutes func(*gin.Engine)

type FHIRServer struct {
	DatabaseHost     string
	Engine           *gin.Engine
	MiddlewareConfig map[string][]gin.HandlerFunc
	AfterRoutes      []AfterRoutes
}

func (f *FHIRServer) AddMiddleware(key string, middleware gin.HandlerFunc) {
	f.MiddlewareConfig[key] = append(f.MiddlewareConfig[key], middleware)
}

func NewServer(databaseHost string) *FHIRServer {
	server := &FHIRServer{DatabaseHost: databaseHost, MiddlewareConfig: make(map[string][]gin.HandlerFunc)}
	server.Engine = gin.Default()

	server.Engine.Use(cors.Middleware(cors.Config{
		Origins:         "*",
		Methods:         "GET, PUT, POST, DELETE",
		RequestHeaders:  "Origin, Authorization, Content-Type, If-Match, If-None-Exist",
		ExposedHeaders:  "Location, ETag, Last-Modified",
		MaxAge:          86400 * time.Second, // Preflight expires after 1 day
		Credentials:     true,
		ValidateHeaders: false,
	}))

	server.Engine.Use(AbortNonJSONRequests)

	return server
}

func (f *FHIRServer) Run(config Config) {
	var err error

	// Setup the database
	session, err := mgo.Dial(f.DatabaseHost)
	if err != nil {
		panic(err)
	}
	log.Println("Connected to mongodb")
	defer session.Close()

	Database = session.DB(config.DatabaseName)

	RegisterRoutes(f.Engine, f.MiddlewareConfig, NewMongoDataAccessLayer(Database), config)

	indexSession := session.Copy()
	ConfigureIndexes(indexSession, config)
	indexSession.Close()

	for _, ar := range f.AfterRoutes {
		ar(f.Engine)
	}

	f.Engine.Run(":3001")
}

// AbortNonJSONRequests is middleware that responds to any request that Accepts a format
// other than JSON with a 406 Not Acceptable status.
func AbortNonJSONRequests(c *gin.Context) {
	acceptHeader := c.Request.Header.Get("Accept")
	if acceptHeader != "" && !strings.Contains(acceptHeader, "json") && !strings.Contains(acceptHeader, "*/*") {
		c.AbortWithStatus(http.StatusNotAcceptable)
	}
}
