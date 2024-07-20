
# hangell/jano
[![godoc](https://godoc.org/github.com/hangell/jano?status.svg)](https://godoc.org/github.com/hangell/jano)
[![sourcegraph](https://sourcegraph.com/github.com/hangell/jano/-/badge.svg)](https://sourcegraph.com/github.com/hangell/jano?badge)

Jano is a Go library that allows you to create HTTP servers with routing similar to Express.js.

## Installation

```sh
go get github.com/hangell/jano
```

## Usage Example

```go
package main

import (
    "log"
    "net/http"
    "os"
    "github.com/hangell/jano"
)

func main() {
    app := jano.New()

    app.Use(loggingMiddleware)

    app.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Welcome to Jano!"))
    })

    app.Post("/login", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Login successful!"))
    })

    app.NotFound(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Custom 404: Page not found"))
    })

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    log.Printf("Listening on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, app.Router()))
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}
```

## Features

- Support for multiple HTTP methods (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `OPTIONS`, `HEAD`)
- Middleware with the `Use` function
- Simple and intuitive routing
- Support for custom 404 handlers with `NotFound`

## Project Structure

```
jano/
│   bench_test.go
│   go.mod
│   jano.go
│   README.md
│
├───.idea
│   │   .gitignore
│   │   jano.iml
│   │   modules.xml
│   │   workspace.xml
│
└───examples
    └───api
        │   main.go
        │   requests.http
        │
        ├───handlers
        │       person_handlers.go
        │
        └───routes
                routes.go
```

## Example API

### Handlers (`examples/api/handlers/person_handlers.go`)

```go
package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "sync"
)

type Person struct {
    ID      int    `json:"id"`
    Name    string `json:"name"`
    Age     int    `json:"age"`
    Student bool   `json:"student"`
}

var (
    people   = make(map[int]Person)
    idCounter = 1
    mutex     = &sync.Mutex{}
)

func GetPeople(w http.ResponseWriter, r *http.Request) {
    mutex.Lock()
    defer mutex.Unlock()
    var peopleList []Person
    for _, person := range people {
        peopleList = append(peopleList, person)
    }
    json.NewEncoder(w).Encode(peopleList)
}

func CreatePerson(w http.ResponseWriter, r *http.Request) {
    var person Person
    if err := json.NewDecoder(r.Body).Decode(&person); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    mutex.Lock()
    defer mutex.Unlock()
    person.ID = idCounter
    idCounter++
    people[person.ID] = person
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(person)
}

func GetPerson(w http.ResponseWriter, r *http.Request) {
    idStr := r.Context().Value("id").(string)
    id, err := strconv.Atoi(idStr)
    if err != nil {
        http.Error(w, "Invalid person ID", http.StatusBadRequest)
        return
    }
    mutex.Lock()
    defer mutex.Unlock()
    person, ok := people[id]
    if !ok {
        http.Error(w, "Person not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(person)
}

func UpdatePerson(w http.ResponseWriter, r *http.Request) {
    idStr := r.Context().Value("id").(string)
    id, err := strconv.Atoi(idStr)
    if err != nil {
        http.Error(w, "Invalid person ID", http.StatusBadRequest)
        return
    }
    var person Person
    if err := json.NewDecoder(r.Body).Decode(&person); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    mutex.Lock()
    defer mutex.Unlock()
    _, ok := people[id]
    if !ok {
        http.Error(w, "Person not found", http.StatusNotFound)
        return
    }
    person.ID = id
    people[id] = person
    json.NewEncoder(w).Encode(person)
}

func DeletePerson(w http.ResponseWriter, r *http.Request) {
    idStr := r.Context().Value("id").(string)
    id, err := strconv.Atoi(idStr)
    if err != nil {
        http.Error(w, "Invalid person ID", http.StatusBadRequest)
        return
    }
    mutex.Lock()
    defer mutex.Unlock()
    if _, ok := people[id]; !ok {
        http.Error(w, "Person not found", http.StatusNotFound)
        return
    }
    delete(people, id)
    w.WriteHeader(http.StatusNoContent)
}
```

### Routes (`examples/api/routes/routes.go`)

```go
package routes

import (
	"log"
	"net/http"
	"github.com/hangell/jano"
	"your_project/handlers"  // Replace with your project path
)

func SetupRoutes(app *jano.Jano) {
	app.Use(loggingMiddleware)
	app.Use(authenticationMiddleware)
	app.Use(corsMiddleware)

	app.Get("/people", handlers.GetPeople)
	app.Post("/people", handlers.CreatePerson)
	app.Get("/people/{id}", handlers.GetPerson)
	app.Put("/people/{id}", handlers.UpdatePerson)
	app.Delete("/people/{id}", handlers.DeletePerson)

	app.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Custom 404: Page not found"))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Example: Check for a specific header or token
		if r.Header.Get("X-Auth-Token") != "secret-token" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

```

### Main (`examples/api/main.go`)

```go
package main

import (
    "log"
    "net/http"
    "os"
    "github.com/hangell/jano"
    "your_project/routes"  // Replace with your project path
)

func main() {
    app := jano.New()

    routes.SetupRoutes(app)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    log.Printf("Listening on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, app.Router()))
}
```

### HTTP Requests (`examples/api/requests.http`)

```http
### Get all people
GET http://localhost:8080/people
Accept: application/json

### Create a new person
POST http://localhost:8080/people
Content-Type: application/json

{
    "name": "John Doe",
    "age": 30,
    "student": false
}

### Get a specific person by ID
GET http://localhost:8080/people/1
Accept: application/json

### Update a person
PUT http://localhost:8080/people/1
Content-Type: application/json

{
    "name": "John Smith",
    "age": 31,
    "student": true
}

### Delete a person
DELETE http://localhost:8080/people/1
```

## Benchmark Results

The following are the results from running the benchmarks:

| Benchmark                        | Operations (ops) | Time per Operation (ns/op) | Memory Allocated (B/op) | Allocations per Operation (allocs/op) |
|----------------------------------|------------------|----------------------------|-------------------------|---------------------------------------|
| BenchmarkJano-8                  | 1,566,112        | 744.9                      | 800                     | 8                                     |
| BenchmarkJanoSimple-8            | 2,936,114        | 410.4                      | 400                     | 4                                     |
| BenchmarkJanoAlternativeInRegexp-8 | 752,006          | 1,478                      | 1,600                   | 16                                    |
| BenchmarkManyPathVariables-8     | 538,432          | 2,160                      | 1,566                   | 25                                    |

## License

This project is licensed under the BSD-3-Clause License - see the LICENSE file for details.

## Donations
If you enjoyed using this project, please consider making a donation to support the continuous development of the project. You can make a donation using one of the following options:
* Pix: rodrigo@hangell.org
* Cryptocurrencies or NFT MetaMask: 0xEd4d1be72F807Faa358C966a8eF63367c200130F

![Created By](https://media.licdn.com/dms/image/D4D03AQF0vBM0rLZMKg/profile-displayphoto-shrink_200_200/0/1704050191664?e=1726099200&v=beta&t=JiPipqyppQaj1f6tR6tI2cMojmCAgJFQXkJgZdAZKqk)

<div>
  <a href="https://hangell.org" target="_blank"><img src="https://img.shields.io/badge/website-000000?style=for-the-badge&logo=About.me&logoColor=white" target="_blank"></a>
  <a href="https://play.google.com/store/apps/dev?id=5606456325281613718" target="_blank"><img src="https://img.shields.io/badge/Google_Play-414141?style=for-the-badge&logo=google-play&logoColor=white" target="_blank"></a>
  <a href="https://www.youtube.com/channel/UC8_zG7RFM2aMhI-p-6zmixw" target="_blank"><img src="https://img.shields.io/badge/YouTube-FF0000?style=for-the-badge&logo=youtube&logoColor=white" target="_blank"></a>
  <a href="https://www.facebook.com/hangell.org" target="_blank"><img src="https://img.shields.io/badge/Facebook-1877F2?style=for-the-badge&logo=facebook&logoColor=white" target="_blank"></a>
  <a href="https://www.linkedin.com/in/rodrigo-rangel-a80810170" target="_blank"><img src="https://img.shields.io/badge/-LinkedIn-%230077B5?style=for-the-badge&logo=linkedin&logoColor=white" target="_blank"></a>
</div>
