package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apex/log"
	"github.com/boltdb/bolt"
	"github.com/dpastoor/nonmemutils/runner"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
)

func main() {

	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	db, err := bolt.Open("models.db", 0600, nil)
	if err != nil {
		log.Fatalf("could not open boltdb with err: %s", err)
	}
	ms := &ModelStore{"path", db}
	fmt.Println("hello ms")
	// create a model bucket to store models
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("models"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	startInsert := time.Now()
	for i := 0; i < 8; i++ {
		newModel := &Model{ID: 0, Status: "Completed",
			Details: ModelDetails{
				ModelPath:   "C://temphello",
				RunSettings: runner.RunSettings{true, "cache.exe"},
				CacheDir:    "cache_dir",
				CacheExe:    "cache.exe",
			}}
		ms.CreateModel(newModel)
	}
	for i := 0; i < 4; i++ {
		newModel := &Model{ID: 0, Status: "Running",
			Details: ModelDetails{
				ModelPath:   "C://temphello",
				RunSettings: runner.RunSettings{true, "cache.exe"},
				CacheDir:    "cache_dir",
				CacheExe:    "cache.exe",
			}}
		ms.CreateModel(newModel)
	}
	for i := 0; i < 10; i++ {
		newModel := &Model{ID: 0, Status: "Queued",
			Details: ModelDetails{
				ModelPath:   "C://temphello",
				RunSettings: runner.RunSettings{true, "cache.exe"},
				CacheDir:    "cache_dir",
				CacheExe:    "cache.exe",
			}}
		ms.CreateModel(newModel)
	}

	fmt.Println("inserted sample model output in: ", time.Since(startInsert))

	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// When a client closes their connection midway through a request, the
	// http.CloseNotifier will cancel the request context (ctx).
	r.Use(middleware.CloseNotify)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	r.Route("/models", func(r chi.Router) {
		r.Get("/", ms.handleGetModels)
		// r.With(paginate).Get("/", listArticles) // GET /articles
		// r.Post("/", createArticle)              // POST /articles
		// r.Get("/search", searchArticles)        // GET /articles/search
		r.Route("/:modelID", func(r chi.Router) {
			r.Use(ms.ModelCtx)
			r.Get("/", ms.handleGetModel) // GET /models/123
		})
	})
	fmt.Println("serving now on 3333")
	http.ListenAndServe(":3333", r)

}

// func getModelStatus(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()
// 	model, ok := ctx.Value("model").(*Model)
// 	if !ok {
// 		http.Error(w, http.StatusText(422), 422)
// 		return
// 	}
// 	w.Write([]byte(fmt.Sprintf("modelID:%v", model.ModelID)))
// }

// func submitModel(w http.ResponseWriter, r *http.Request) {
// 	model := new(Model)
// 	fmt.Println("new model submitted to queue")
// 	w.Write([]byte(fmt.Sprintf("modelID:%v", model.ModelID)))
// }

// func dbGetModel(aid string) (*Model, error) {
// 	return new(Model), nil
// }