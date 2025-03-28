package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"	
	"io"
	"log"

	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
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


func restDeletePost(w http.ResponseWriter, r *http.Request){
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	del := deletePost(id)
	if del {
		msg := fmt.Sprint(id)+ " id deleted"
		json.NewEncoder(w).Encode(msg)
		return 
	}
	http.Error(w, "can't deleted the post", http.StatusNotFound)
}




type SoapRequest struct {
	XMLName xml.Name `xml:"CreatePost"`
	Title string `xml:"title"`
	Content string `xml:"content"`
}


type SoapResponse struct {
	XMLName xml.Name `xml:"CreatePostResponse"`
	Post Post 	`xml:"post"`
}

func soapHandler(w http.ResponseWriter, r *http.Request){
	body, _ := io.ReadAll(r.Body)
	var req SoapRequest
	xml.Unmarshal(body, &req)
	p := createPost(req.Title, req.Content)
	resp := SoapResponse{Post: p}
	w.Header().Set("Content-Type", "text/xml")
	xml.NewEncoder(w).Encode(resp)
}






// graphql


var postType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Post",
	Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.Int},
		"title": &graphql.Field{Type: graphql.String},
		"content": &graphql.Field{Type: graphql.String},

	},
})



var queryType =  graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"posts": &graphql.Field{
			Type: graphql.NewList(postType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return listPosts(), nil
			},
		},
	},
})


var mutationType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Mutation",
	Fields: graphql.Fields{
		"createPost": &graphql.Field{
			Type: postType,
			Args: graphql.FieldConfigArgument{
				"title": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"content": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return createPost(p.Args["title"].(string), p.Args["content"].(string)), nil
			},
		},
	},
})


var schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: queryType,
	Mutation: mutationType,
})


func graphqlHandler(w http.ResponseWriter, r * http.Request){
	var params struct{
		Query string 
	}
	json.NewDecoder(r.Body).Decode(&params)
	result := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: params.Query,
	})
	json.NewEncoder(w).Encode(result)
}







func restServer(){
	r := mux.NewRouter()
	r.HandleFunc("/api/posts", restListPosts).Methods("GET")
	r.HandleFunc("/api/posts/{id}", restGetPost).Methods("GET")
	r.HandleFunc("/api/posts/{id}", restUpdatePost).Methods("PUT")
	r.HandleFunc("/api/posts/{id}", restDeletePost).Methods("DELETE")
	r.HandleFunc("/api/posts", restCreatePost).Methods("POST")
	go http.ListenAndServe(":8080", r)

}




func soapServer(){
	http.HandleFunc("/soap", soapHandler)
	go http.ListenAndServe(":8081", nil)
}






func graphqlServer(){
	
}


var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
}




func wsHandler(w http.ResponseWriter, r * http.Request){
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Panicln("ws upgrade failed: ", err)
		return 
	}

	defer conn.Close()

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			clientsMu.Lock()
			delete(clients, conn)
			clientsMu.Unlock()
			break
		}
	}

}

func broadcastPosts(){
	for post := range broadcast{
		clientsMu.Lock()
		for client := range clients{
			err := client.WriteJSON(post)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
	clientsMu.Unlock()
}



func grpcServer(){

}


func wsServer(){
	http.HandleFunc("/ws", wsHandler)
	go http.ListenAndServe(":8083", nil)
	go broadcastPosts()
}

func  main() {
	// REST
	restServer()
	
	// GRPC
	grpcServer()


	// SOAP
	soapServer()

	// GRAPHQL

	graphqlServer()

	// WEBSOCKET
	wsServer()

	fmt.Println("Server running:\n\tREST(:8080)\n\tSOAP(:8081)\n\tGraphQL(:8082)\n\tWebsocket(:8083)\n\tgRPC(:50051)")

	select{}



}