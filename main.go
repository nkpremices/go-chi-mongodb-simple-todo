package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"context"
	"os"
	"os/signal"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"github.com/nkpremices/go-chi-mongodb-simple-todo/src/utils"
)

var rnd *renderer.Render
var db *mgo.Database

const (
	hostName				string = "localhost:27017"
	dbName					string = "demo_todo"
	collectionName			string = "Todo"
	port					string = ":9000"
)

type(
	TodoModel struct {
		ID				bson.ObjectId `bson:"_id,omitempty"`
		Title			string `bson:"title"`
		Completed		bool `bson:"completed"`
		CreatedAt		time.Time `bson:"createdAt"`
	}

	Todo struct {
		ID				string `json:"id"`
		Title			string `json:"title"`
	    Completed		bool `json:"completed"`
		CreatedAt		time.Time `json:"createdAt"`
	}
)

func init() {
	rnd = renderer.New()
	sess, err := mgo.Dial(hostName)
	utils.CheckErr(err)
	sess.SetMode(mgo.Monotonic, true)

	db = sess.DB(dbName)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	utils.CheckErr(err)
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
	var todos []TodoModel

	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		jsonErr := rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to fetch Todo",
			"error": err,
		})

		utils.CheckErr(jsonErr)
		return
	}
	var todoList []Todo

	for _, t := range todos {
		todoList = append(todoList, Todo{
			ID: t.ID.Hex(),
			Title: t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t Todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonErr := rnd.JSON(w, http.StatusProcessing, err)
		utils.CheckErr(jsonErr)
		return
	}

	if t.Title == "" {
		jsonErr := rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title is required",
		})
		utils.CheckErr(jsonErr)
		return
	}

	tm := TodoModel{
		ID: bson.NewObjectId(),
		Title: t.Title,
		Completed: false,
		CreatedAt: time.Now(),
	}

	if err := db.C(collectionName).Insert(&tm); err != nil {
		jsonErr := rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to save todo",
		})
		utils.CheckErr(jsonErr)
		return
	}

	jsonErr := rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "todo created successfully",
		"todo_id": tm.ID.Hex(),
	})

	utils.CheckErr(jsonErr)
	return
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		jsonErr := rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The id is invalid",
		})

		utils.CheckErr(jsonErr)
		return
	}

	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		jsonErr := rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to delete todo",
			"error": err,
		})

		utils.CheckErr(jsonErr)
		return
	}

	jsonErr := rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})

	utils.CheckErr(jsonErr)
	return
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		jsonErr := rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The id is invalid",
		})

		utils.CheckErr(jsonErr)
		return
	}

	var t Todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonErr := rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "The body is invalid",
			"error": err,
		})

		utils.CheckErr(jsonErr)
		return
	}

	if t.Title == "" {
		jsonErr := rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title field is required",
		})

		utils.CheckErr(jsonErr)
		return
	}

	if err := db.C(collectionName).Update(bson.M{
		"_id": bson.ObjectIdHex(id),
	},
	bson.M{
		"title": t.Title, "completed": t.Completed,
	});
	err != nil {
		jsonErr := rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to update todo",
		})

		utils.CheckErr(jsonErr)
		return
	}
}

func main()  {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", homeHandler)

	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr: port,
		Handler: r,
		ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Println("listening on port", port)
		if err:=srv.ListenAndServe(); err!=nil {
			log.Printf("listen:%s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)

	defer cancel()
		log.Println("server gracefully stopped")
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()

	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})

	return rg
}
