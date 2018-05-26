package main

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"gopkg.in/olivere/elastic.v3"
	"net/http"
	"reflect"
	"time"
	"strings"
)

const (
	TYPE_USER = "user"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Age      int    `json:"age"`
	Gender   string `json:"gender"`
}

//check whether a pair of username and password is stored in ES.
func checkUser(username, password string) bool {
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		fmt.Printf("ES is not setup %v\n", err)
		return false
	}

	// Search with a term query
	termQuery := elastic.NewMatchQuery("username", username)
	queryResult, err := es_client.Search().
		Index(INDEX).
		Query(termQuery).
		Pretty(true).
		Do()
	if err != nil {
			fmt.Printf("ES query failed %v\n", err)
			return false
		}


	var tyu User
	for _, item := range queryResult.Each(reflect.TypeOf(tyu)) {
		u := item.(User)
		return u.Password == password && u.Username == username
	}
	// If no user exist, return false.
	return false
}

// 1) After we send the term query, how do we know whether this user has existed?
// 2) insert the user to ES
func addUser(user User) bool {
	// In theory, BigTable is a better option for storing user credentials than ES. However,
  // since BT is more expensive than ES so usually students will disable BT.
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))

	if err != nil {
		fmt.Printf("ES is not setup %v\n", err)
		return false
	}

	//Search in es first to check if the user already exist
	termQuery := elastic.NewMatchQuery("username", user.Username)
	queryResult, err := es_client.Search().Index(INDEX).Query(termQuery).Pretty(true).Do()
	// queryResult, err := es_client.Search().
	// 					Index(INDEX).
	// 					Query(termQuery).
	// 					Pretty(true).
	// 					Do()

	if err != nil {
		fmt.Printf("ES query failed %v\n", err)
		return false
	}

	if queryResult.TotalHits() > 0 {
		fmt.Printf("User %v has existed, cannot create duplicate Users", user.Username)
		return false
	}

	//Create user if there is no duplicate
	_, err1 := es_client.Index().
		Index(INDEX).
		Type(TYPE_USER).
		Id(user.Username).
		BodyJson(user).
		Refresh(true).
		Do()

	if err1 != nil {
		fmt.Printf("ES save failed %v\n", err)
		return false
	}
	return true
}

// 1) Decode a user from request (POST)
// 2) Check whether username and password are empty, if any of them is empty,
//    call http.Error(w, "Empty password or username", http.StatusInternalServerError)
// 3) Otherwise, call addUser, if true, return a message “User added successfully”
// 4) If else, call http.Error(w, "Failed to add a new user", http.StatusInternalServerError)
// 5) Set header to be  w.Header().Set("Content-Type", "text/plain")
//    w.Header().Set("Access-Control-Allow-Origin", "*")
// If signup is successful, a new session is created.
func signupHandler(w http.ResponseWriter, r *http.Request) {
      fmt.Println("Received one signup request")

      decoder := json.NewDecoder(r.Body)
      var u User
      if err := decoder.Decode(&u); err != nil {
             panic(err)
             return
      }
      u.Username = strings.ToLower(u.Username)


      if u.Username != "" && u.Password != "" {
             if addUser(u) {
                    fmt.Println("User added successfully.")
                    w.Write([]byte("User added successfully."))
             } else {
                    fmt.Println("Failed to add a new user.")
                    http.Error(w, "Failed to add a new user", http.StatusInternalServerError)
             }
      } else {
             fmt.Println("Empty password or username.")
             http.Error(w, "Empty password or username", http.StatusInternalServerError)
      }

      w.Header().Set("Content-Type", "text/plain")
      w.Header().Set("Access-Control-Allow-Origin", "*")
}


// If login is successful, a new token is created.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one login request")
  //Decode a user from request’s body
	decoder := json.NewDecoder(r.Body)
	var u User
	if err := decoder.Decode(&u); err != nil {
		panic(err)
		return
	}
  //Make sure user credential is correct.
	if checkUser(u.Username, u.Password) {
    //Create a new token object to store.
		token := jwt.New(jwt.SigningMethodHS256)
    //Convert it into a map for lookup
		claims := token.Claims.(jwt.MapClaims)
		/* Set token claims */
    //Store username and expiration into it.
		claims["username"] = u.Username
		claims["exp"] = time.Now().Add(time.Hour * 24).Unix() //oi that overflow

		/* Sign the token with our secret */
    //Sign (Encrypt) and token such that only server knows it.
		tokenString, _ := token.SignedString(mySigningKey)

		/* Finally, write the token to the browser window */
    //Write it into response
		w.Write([]byte(tokenString))
	} else {
		fmt.Println("Invalid password or username.")
		http.Error(w, "Invalid password or username", http.StatusForbidden)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
