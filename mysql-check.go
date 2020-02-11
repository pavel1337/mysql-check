package main

import (
	"database/sql"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/ghodss/yaml"
	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	MySQLAddress      string `json:"mysql_address"`
	MySQLTimeout      string `json:"mysql_timeout"`
	MySQLUserPassword string `json:"mysql_user_password"`
	HttpAddress       string `json:"http_address"`
}

type application struct {
	debug    bool
	infoLog  *log.Logger
	errorLog *log.Logger
	dsn      string
}

func main() {
	path := flag.String("c", "config.yml", "path to the config")
	debug := flag.Bool("d", false, "debug messages")
	flag.Parse()
	c, err := parseConfig(*path)
	if err != nil {
		log.Println(err)
		return
	}
	dsn := c.MySQLUserPassword + "@tcp(" + c.MySQLAddress + ")/?timeout=" + c.MySQLTimeout + "s"
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

	app := &application{
		debug:    *debug,
		dsn:      dsn,
		errorLog: errorLog,
		infoLog:  infoLog,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.check)

	err = http.ListenAndServe(c.HttpAddress, mux)
	if err != nil {
		log.Fatal(err)
	}
}

func (app *application) check(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	db, err := sql.Open("mysql", app.dsn)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		app.serverError(w, err)
		return
	}
	if err = queryReadOnly(db); err != nil {
		app.serverError(w, err)
		return
	}
	if app.debug {
		app.infoLog.Println("check is ok")
	}
	return
}

func parseConfig(p string) (Config, error) {
	c := Config{}
	rawConfig, err := ioutil.ReadFile(p)
	if err != nil {
		flag.Usage()
		return c, err
	}
	err = yaml.Unmarshal(rawConfig, &c)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (app *application) serverError(w http.ResponseWriter, err error) {
	if app.debug {
		app.errorLog.Output(2, err.Error())
	}
	http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
}

func queryReadOnly(db *sql.DB) error {
	var s int
	row := db.QueryRow("SELECT @@global.read_only;")
	err := row.Scan(&s)
	if err != nil {
		return err
	}
	if s != 0 {
		return errors.New("mysql is read only")
	}
	return nil
}
