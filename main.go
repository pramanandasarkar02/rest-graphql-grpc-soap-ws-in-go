package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)




type Post struct{
	ID 	int `json:"id" xml:"id"`
	Title string `json:"title" xml:"title"`
	Content string `json:"content" xml:"content"`
}


var (
	posts []Post
	nextID = 1
	mu sync.Mutex
	clients = make(map[*websocket.Conn]bool)
	broadcast = make(chan Post)
	clientsMu sync.Mutex
)



// CRUD logic

func createPost(title, content string) Post{
	mu.Lock()
	defer mu.Unlock()
	
	post := Post{
		ID: nextID,
		Title: title,
		Content: content,
	}
	nextID++
	posts = append(posts, post)
	// broadcast <- post
	return post
}



func getPost(id int)(Post, bool) {
	for _ , p := range posts{
		if p.ID == id{
			return p, true
		}
	} 
	return Post{}, false
}


func listPosts() []Post {
	return posts
}



func updatePost(id int, title, content string)(Post, bool){
	mu.Lock()
	defer mu.Unlock()

	for i, p := range posts{
		if p.ID == id{
			posts[i] = Post{
				ID: id,
				Title: title,
				Content: content,
			}
			return posts[i], true
		}
	}
	return Post{}, false
}


func deletePost(id int) bool{
	for i, p := range posts{
		if p.ID == id{
			posts = append(posts[:i], posts[i+1:]... )
			return true
		}
	}
	return false
}




// REST Handlers


func restCreatePost(w http.ResponseWriter, r *http.Request){
	var p Post
	json.NewDecoder(r.Body).Decode(&p)
	post := createPost(p.Title, p.Content)
	json.NewEncoder(w).Encode(post)
}


func restGetPost(w http.ResponseWriter, r *http.Request){
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	if p, found := getPost(id); found{
		json.NewEncoder(w).Encode(p)
		return
	}
	http.Error(w, "Post not found", http.StatusNotFound)
}


func restListPosts(w http.ResponseWriter, r *http.Request){
	json.NewEncoder(w).Encode(listPosts())
}


func restUpdatePost(w http.ResponseWriter, r *http.Request){
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var p Post
	json.NewDecoder(r.Body).Decode(&p)
	post, found := updatePost(id, p.Title, p.Content)
	if found{
		json.NewEncoder(w).Encode(post)
		return
	}
	http.Error(w, "post didn't update", http.StatusNotFound)
}







func restServer(){
	r := mux.NewRouter()
	r.HandleFunc("/api/posts", restListPosts).Methods("GET")
	r.HandleFunc("/api/posts/{id}", restGetPost).Methods("GET")
	r.HandleFunc("/api/posts/{id}", restUpdatePost).Methods("PUT")
	r.HandleFunc("/api/posts", restCreatePost).Methods("POST")
	go http.ListenAndServe(":8080", r)

}


func  main() {
	// REST
	restServer()
	
	// GRPC


	// SOAP


	// GRAPHQL

	// WEBSOCKET

	fmt.Println("Server running:\n\tREST(:8080)")

	select{}



}