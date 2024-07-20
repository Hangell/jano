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
