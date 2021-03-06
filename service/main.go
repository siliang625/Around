package main

import (
	"cloud.google.com/go/bigtable"  //big table
	"cloud.google.com/go/storage"  //to upload image  GCS
	"context"   								  	//to upload image
	"encoding/json"
	"net/http"
	"fmt"
	"github.com/auth0/go-jwt-middleware"  //auth
	"github.com/dgrijalva/jwt-go"  //auth
	"github.com/gorilla/mux"  //auth
	"github.com/pborman/uuid"
	"gopkg.in/olivere/elastic.v3"    //ES
	"io"
	"log"
	"reflect"
	"strconv"
	"strings"
)

const (
	INDEX    = "around"
	TYPE     = "post"
	DISTANCE = "200km"
	//TODO: update the following information when deploing
	ES_URL = "http://35.237.20.173:9200"
	BUCKET_NAME = "post-image-204022"
	PROJECT_ID  = "around-204022"
	BT_INSTANCE = "around-post"
)

type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Post struct {
	// `json:"user"` is for the json parsing of this User field. Otherwise, by default it's 'User'.
	User     string   `json:"user"`
	Message  string   `json:"message"`
	Location Location `json:"location"`
	Id       string   `json:"id"`
	Url      string   `json:"url"`
}

var mySigningKey = []byte("secret")

func main() {
	//Start from scratch: deleteIndex()
	// In order to create index in ES, we need to update main
	// Create a client
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Use the IndexExists service to check if a specified index exists.
	exists, err := client.IndexExists(INDEX).Do()
	if err != nil {
		panic(err)
	}
	// If not, create a new mapping. For other fields (user, message, etc.)
	// no need to have mapping as they are default. For geo location (lat, lon),
	// we need to tell ES that they are geo points instead of two float points
	// such that ES will use Geo-indexing for them (K-D tree)
	if !exists {
		mapping := `{
                    "mappings":{
                           "post":{
                                  "properties":{
                                         "location":{
                                                "type":"geo_point"
                                         }
                                  }
                           }
                    }
             }
             `
		 // Create this index
		_, err := client.CreateIndex(INDEX).Body(mapping).Do()
		if err != nil {
			// Handle error
			panic(err)
		}
	}

	fmt.Println("started-service")
	// Create a new router on top of the existing http router as we need to check auth.
	// Here we are instantiating the gorilla/mux router
	r := mux.NewRouter()

	//Create a new JWT middleware with a Option that uses the
	//key ‘mySigningKey’ such that we know this token is from our server.
	//The signing method is the default HS256 algorithm such that data is encrypted.
	var jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return mySigningKey, nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
  //use jwt middleware to manage these endpoints and if they don’t have valid token,
	// we will reject them: reject code returned: https://golang.org/src/net/http/status.go
	r.Handle("/post", jwtMiddleware.Handler(http.HandlerFunc(handlerPost))).Methods("POST")
	r.Handle("/search", jwtMiddleware.Handler(http.HandlerFunc(handlerSearch))).Methods("GET")
	r.Handle("/login", http.HandlerFunc(loginHandler)).Methods("POST")
	r.Handle("/signup", http.HandlerFunc(signupHandler)).Methods("POST")
	//TODO: r.Handle("/delete",xxx)

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8080", nil))

}

//construct a Post object p to hold a user's post request
func handlerPost(w http.ResponseWriter, r *http.Request) {
	// Parse from body of request to get a json object.
	// to populate username
	user := r.Context().Value("user")
	claims := user.(*jwt.Token).Claims
	username := claims.(jwt.MapClaims)["username"]

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")

	// 32 << 20 is the maxMemory param for ParseMultipartForm, equals to 32MB (1MB = 1024 * 1024 bytes = 2^20 bytes)
	// After you call ParseMultipartForm, the file will be saved in the server memory with maxMemory size.
	// If the file size is larger than maxMemory, the rest of the data will be saved in a system temporary file.
	r.ParseMultipartForm(32 << 20)

	// Parse from form data.
	fmt.Printf("Received one post request %s\n", r.FormValue("message"))
	lat, _ := strconv.ParseFloat(r.FormValue("lat"), 64)
	lon, _ := strconv.ParseFloat(r.FormValue("lon"), 64)
	p := &Post{
		User:    username.(string),
		Message: r.FormValue("message"),
		Location: Location{
			Lat: lat,
			Lon: lon,
		},
	}
  // add a new document to the index
	//TODO: Append its unique ID.
	id := uuid.New()
	p.Id = id

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Image is not available", http.StatusInternalServerError)
		fmt.Printf("Image is not available %v.\n", err)
		return
	}
	defer file.Close()
	ctx := context.Background()

	// replace it with real bucket name.
	_, attrs, err := saveToGCS(ctx, file, BUCKET_NAME, id)
	if err != nil {
		http.Error(w, "GCS is not setup", http.StatusInternalServerError)
		fmt.Printf("GCS is not setup %v\n", err)
		return
	}

	// Update the media link after saving to GCS.
	p.Url = attrs.MediaLink

	//Save to ES
	saveToES(p, id)

	// Save to BigTable.
	saveToBigTable(ctx, p, id)

}

func handlerDelete(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one delete request")
	decoder := json.NewDecoder(r.Body)
	var p Post
	if err := decoder.Decode(&p); err != nil {
		panic(err)
		return
	}
	deleteToES(&p)
	ctx := context.Background()
	_, err := deleteToGCS(ctx, BUCKET_NAME, p.Id)
	if err != nil {
		http.Error(w, "The deletion of %s is not successful.\n", http.StatusInternalServerError)
		fmt.Printf("The deletion of %s is not successful.\n", p.Id)
		return
	}

	//TODO: deleteToGCS
	//TODO: deleteToBigTable
}

// Save a post to ElasticSearch
func saveToES(p *Post, id string) {
	// Create a client
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Save it to index
	_, err = es_client.Index().
		Index(INDEX).
		Type(TYPE).
		Id(id).
		BodyJson(p).
		Refresh(true).
		Do()
	if err != nil {
		panic(err)
		return
	}

	fmt.Printf("Post is saved to Index: %s\n", p.Message)
}

func saveToBigTable(ctx context.Context, p *Post, id string) {
	//Create a client to bigTable
	bt_client, err := bigtable.NewClient(ctx, PROJECT_ID, BT_INSTANCE)
	if err != nil {
		panic(err)
		return
	}

	tbl := bt_client.Open("post")
	mut := bigtable.NewMutation()
	t := bigtable.Now()

	//save data to a row(mutation)
	//TODO: edit the table structure under each post to include the id field.
	mut.Set("post", "user", t, []byte(p.User)) //like "jack1"
	mut.Set("post", "message", t, []byte(p.Message))
	mut.Set("location", "lat", t, []byte(strconv.FormatFloat(p.Location.Lat, 'f', -1, 64)))
	mut.Set("location", "lon", t, []byte(strconv.FormatFloat(p.Location.Lon, 'f', -1, 64)))

	err = tbl.Apply(ctx, id, mut)
	if err != nil {
		panic(err)
		return
	}
	fmt.Printf("Post is saved to BigTable: %s\n", p.Message)

}

//save an image to GCS
func saveToGCS(ctx context.Context, r io.Reader, bucketName, name string) (*storage.ObjectHandle, *storage.ObjectAttrs, error) {
	// Student questions
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	// Next check if the bucket exists
	if _, err = bucket.Attrs(ctx); err != nil {
		return nil, nil, err
	}

	obj := bucket.Object(name)
	w := obj.NewWriter(ctx)
	if _, err := io.Copy(w, r); err != nil {
		return nil, nil, err
	}
	if err := w.Close(); err != nil {
		return nil, nil, err
	}

	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		return nil, nil, err
	}

	attrs, err := obj.Attrs(ctx)
	fmt.Printf("Post is saved to GCS: %s\n", attrs.MediaLink)
	return obj, attrs, err

}

func deleteToES(p *Post) {
	// Create a client
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Delete by Query
	id := p.Id
	_, err = es_client.Delete().Index(INDEX).Type(TYPE).Id(id).Do()

	if err != nil {
		panic(err)
		return
	}

	fmt.Printf("Post is deleted to Index: %s\n", p.Message)
}

func deleteToGCS(ctx context.Context, bucketName, name string) (bool, error) {
	client, err := storage.NewClient(ctx)

	if err != nil {
		return false, err
	}

	defer client.Close()

	bucket := client.Bucket(bucketName)
	if _, err := bucket.Attrs(ctx); err != nil {
		return false, err
	}

	obj := bucket.Object(name)
	if err := obj.Delete(ctx); err != nil {
		return false, err
	}

	return true, nil

}

func deleteIndex() {
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	_, err = es_client.DeleteIndex(INDEX).Do()

	if err != nil {
		panic(err)
		return
	}

	fmt.Printf("Index is deleted. Restart service to recreate mapping.\n")
}

//
func handlerSearch(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one request for search")
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, _ := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	// range is optional
	ran := DISTANCE
	//to get request parameters from url: lat := r.URL.Query().Get("lat")
	if val := r.URL.Query().Get("range"); val != "" {
		ran = val + "km"
	}

	fmt.Printf("Search received: %f %f %s\n", lat, lon, ran)

	// Create a client: create a connection to ES,
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
  //If there is err, return
	if err != nil {
		panic(err)
		return
	}

  // Prepare a geo based query to find posts within a geo box.
	// https://www.elastic.co/guide/en/elasticsearch/reference/5.2/query-dsl-geo-distance-query.html
	q := elastic.NewGeoDistanceQuery("location")
	q = q.Distance(ran).Lat(lat).Lon(lon)

  //Get the results based on Index and query (q)
	// Some delay may range from seconds to minutes.
	// So if you don't get enough results. Try it later.
	searchResult, err := client.Search().
		Index(INDEX).
		Query(q).
		Pretty(true).  //format output
		Do()
		if err != nil {
			// Handle error
			panic(err)
		}


	// searchResult is of type SearchResult and returns hits, suggestions,
	// and all kinds of other information from Elasticsearch.
	fmt.Printf("Query took %d milliseconds\n", searchResult.TookInMillis)
	// TotalHits is another convenience function that works even when something goes wrong.
	fmt.Printf("Found a total of %d post\n", searchResult.TotalHits())

	// Each is a convenience function that iterates over hits in a search result.
	// It makes sure you don't need to check for nil values in the response.
	// However, it ignores errors in serialization.
	var typ Post
	var ps []Post
	//Iterate results and if they are type of Post (typ)
	for _, item := range searchResult.Each(reflect.TypeOf(typ)) { // instance of
		// Cast an item to Post
		p := item.(Post) // p = (Post) item
		fmt.Printf("Post by %s: %s at lat %v and lon %v\n", p.User, p.Message, p.Location.Lat, p.Location.Lon)
		/* TODO：Perform filtering based on keywords such as web spam etc.
		if !containsFilteredWords(&p.Message){
			ps = append(ps, p)
		}
		*/
		//Add the p to an array, equals ps.add(p) in java
		ps = append(ps, p)
	}
	//Convert the go object to a string
	js, err := json.Marshal(ps)
	if err != nil {
		panic(err)
		return
	}
  //Allow cross domain visit for javascript.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)

}

//TODO: use regex for better filtering.
func containsFilteredWords(s *string) bool {
	filteredWords := []string{
		"fuck",
		"shit",
	}
	for _, word := range filteredWords {
		if strings.Contains(*s, word) {
			return true
		}
	}
	return false
}
