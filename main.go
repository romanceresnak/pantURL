package main

import (
	"crypto/sha1"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
)

var SQLITE_FILE_NAME = "./shortenedURLs.db"
var SQLITE_DSN = "file:" + SQLITE_FILE_NAME + "?mode=rwc"
var HTTP_PORT = ":8080"
var db *sql.DB
var urlLookupQuery *sql.Stmt

func init() {
	httpInit()
	err := sqlInit()
	if err != nil {
		log.Fatal(err)
	}
}

func httpInit() {
	http.HandleFunc("/", homePageHandler)
	http.HandleFunc("/submit", postHandler)
}

func shutdown() {
	urlLookupQuery.Close()
	db.Close()
}

func sqlInit() error {
	var err error
	db, err = sql.Open("sqlite3", SQLITE_DSN)
	if err != nil {
		return err
	}

	sqlStmt := `
	 CREATE TABLE IF NOT EXISTS urls
  (
     shortcode TEXT NOT NULL PRIMARY KEY,
     url       TEXT NOT NULL
  );`
	_, err = db.Exec(sqlStmt)

	urlLookupQuery, err = db.Prepare("SELECT url FROM urls WHERE shortcode = ?")
	if err != nil {
		return err
	}

	return nil
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	if (r.URL.Path == "/") {
		fmt.Fprintf(w, pageHeader()+homepageContent()+pageFooter())
		return
	}

	//lookup
	inputShortcode := r.URL.Path[1:]
	var targetUrl string
	var err = urlLookupQuery.QueryRow(inputShortcode).Scan(&targetUrl)
	if err != nil {
		response := `
        <h1>shortURL</h1>
		<div>Sorry we encountered an error</div>
        <p>` + err.Error() + "</p>"

		fmt.Fprintf(w, pageHeader()+response+pageFooter())
		return
	}

	//redirect
	http.Redirect(w, r, targetUrl, 301)
	log.Printf("Redirecting user with shortcode %s to %s", inputShortcode, targetUrl)
}

func stringToSHA1(input string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(input)))
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, pageHeader())
	defer fmt.Fprintf(w, pageFooter())

	tx, err := db.Begin()
	if err != nil {
		log.Print(err)
		fmt.Fprintf(w, "DB transaction begin err: %v", err)
		return
	}
	stmt, err := tx.Prepare("insert into urls(shortcode, url) values(?, ?)")
	if err != nil {
		log.Print(err)
		fmt.Fprintf(w, "DB transaction prepare err: %v", err)
		return
	}
	defer stmt.Close()

	if err := r.ParseForm(); err != nil {
		log.Print(err)
		fmt.Fprintf(w, "Request ParseForm() err: %v", err)
		return
	}
	inputUrl := r.FormValue("urlField")
	shortcode := stringToSHA1(inputUrl)[:8] //only care about first 8 chars
	_, err = stmt.Exec(shortcode, inputUrl)
	if err != nil {
		log.Print(err)
		fmt.Fprintf(w, "DB Insert err: %v", err)
		return
	}
	tx.Commit()

	log.Printf("Saved url '%s' as shortcode '%s'", inputUrl, shortcode)
	completeUrl := r.Host + "/" + shortcode
	fmt.Fprintf(w, "Your new shortened url to %s is <a href=\"%s\">%s</a>", inputUrl, completeUrl, completeUrl)
}

func main() {
	log.Fatal(http.ListenAndServe(HTTP_PORT, nil))
	shutdown()
}
