package main

import (
	"fmt"
	"forum/api"
	"forum/controllers"
	"forum/pkgs/funcs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ClientLimiter struct {
	limiter map[string]*rate.Limiter
	mu      sync.Mutex
}

func newClientLimiter() *ClientLimiter {
	return &ClientLimiter{
		limiter: make(map[string]*rate.Limiter),
	}
}

func (cl *ClientLimiter) getLimiter(ip string) *rate.Limiter {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	limiter, exists := cl.limiter[ip]
	if !exists {
		limiter = rate.NewLimiter(2, 5) // Adjust rate and burst as needed
		cl.limiter[ip] = limiter
	}

	return limiter
}

var clientLimiter = newClientLimiter()

func main() {
	funcs.Init()

	// Create a file server to serve static files (CSS, JS, images, etc.)
	fs := http.FileServer(http.Dir("static"))

	// Handle requests for files in the "/static/" path
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	rateLimitMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			limiter := clientLimiter.getLimiter(ip)

			if !limiter.Allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	// API endpoints
	http.Handle("/signup", rateLimitMiddleware(http.HandlerFunc(api.SignUp)))                               // Handle signup
	http.Handle("/login", rateLimitMiddleware(http.HandlerFunc(api.LogIn)))                                 // Handle login
	http.Handle("/api/create_post", rateLimitMiddleware(http.HandlerFunc(api.Create_Post)))                 // create post
	http.Handle("/api/create_category", rateLimitMiddleware(http.HandlerFunc(api.Create_Category_Handler))) // create category
	http.Handle("/api/add_comment", rateLimitMiddleware(http.HandlerFunc(api.AddCommentHandler)))           // Handle Create comment
	http.Handle("/api/likes_post", rateLimitMiddleware(http.HandlerFunc(api.LikesPostHandler)))             // Handle Likes & Dislikes for Posts
	http.Handle("/api/likes_comment", rateLimitMiddleware(http.HandlerFunc(api.LikesCommentHandler)))       // Handle Likes & Dislikes for Posts
	http.Handle("/api/posts", rateLimitMiddleware(http.HandlerFunc(api.GetPostsHandler)))                   // Retrieve posts as JSON
	http.Handle("/api/post/", rateLimitMiddleware(http.HandlerFunc(api.Get_post_handler)))                  // Retrieve one post ex: /post/2
	http.Handle("/api/comments", rateLimitMiddleware(http.HandlerFunc(api.Serve_comments_handler)))         // Serves post comments

	// Render pages
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		controllers.RenderPage(w, r, funcs.DB)
	})
	http.HandleFunc("/category/", func(w http.ResponseWriter, r *http.Request) {
		controllers.RenderCategoryPage(w, r, funcs.DB)
	})
	http.HandleFunc("/post/", func(w http.ResponseWriter, r *http.Request) {
		controllers.RenderPostPage(w, r, funcs.DB)
	})
	http.HandleFunc("/user/", func(w http.ResponseWriter, r *http.Request) {
		controllers.RenderUserPage(w, r, funcs.DB)
	})

	// Use your actual SSL certificate and private key filenames
	certFile := "cert.pem"
	keyFile := "key.pem"

	// Set reduced timeouts for read, write, and idle
	server := &http.Server{
		Addr:         ":8080",
		TLSConfig:    nil,              // Use default TLS configuration
		ReadTimeout:  5 * time.Second,  // Reduce read timeout to 5 seconds
		WriteTimeout: 5 * time.Second,  // Reduce write timeout to 5 seconds
		IdleTimeout:  10 * time.Second, // Reduce idle timeout to 10 seconds
	}

	fmt.Println("Server listening on port https://localhost:8080 ...")
	log.Fatal(server.ListenAndServeTLS(certFile, keyFile))

	if err := funcs.DB.Close(); err != nil {
		fmt.Println("Error:", err)
	}
}
