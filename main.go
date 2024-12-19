package main

import (
    "context"
    "encoding/json"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/mux"
    "github.com/sirupsen/logrus"
)

// Response structure for JSON responses
type Response struct {
    Message string `json:"message"`
}

// helloHandler responds with a plain text message
func helloHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Hello, World!"))
}

// jsonHandler responds with a JSON message
func jsonHandler(w http.ResponseWriter, r *http.Request) {
    response := Response{Message: "Hello, JSON World!"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// loggingMiddleware logs incoming HTTP requests
func loggingMiddleware(logger *logrus.Logger) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            logger.Infof("Started %s %s", r.Method, r.RequestURI)
            start := time.Now()
            next.ServeHTTP(w, r)
            duration := time.Since(start)
            logger.Infof("Completed %s in %v", r.RequestURI, duration)
        })
    }
}

// recoveryMiddleware recovers from panics and logs the error
func recoveryMiddleware(logger *logrus.Logger) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    logger.Errorf("Panic: %v", err)
                    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}

func main() {
    // Initialize logger
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{})
    logger.SetOutput(os.Stdout)
    logger.SetLevel(logrus.InfoLevel)

    // Create a new router
    router := mux.NewRouter()

    // Apply middleware
    router.Use(loggingMiddleware(logger))
    router.Use(recoveryMiddleware(logger))

    // Define routes
    router.HandleFunc("/", helloHandler).Methods("GET")
    router.HandleFunc("/json", jsonHandler).Methods("GET")

    // Read port from environment variable or default to 8080
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    addr := ":" + port

    srv := &http.Server{
        Handler:      router,
        Addr:         addr,
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Start server in a goroutine
    go func() {
        logger.Infof("Starting server on %s", addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatalf("Server failed to start: %v", err)
        }
    }()

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
    logger.Info("Shutting down server...")

    // Create a deadline to wait for
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        logger.Fatalf("Server forced to shutdown: %v", err)
    }

    logger.Info("Server exiting")
}
