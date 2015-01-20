package cfg

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"log/syslog"
	"strconv"
)

type Config struct {
	DataBase  DataBase
	WebServer WebServer
	SQS       SQS
}

type DataBase struct {
	Host              string // database host name.  Often `localhost`.
	Name              string // the name of the database to connect to on the host e.g., `fits`
	User              string // user to connect to the database.
	Password          string // password (unencrypted) for the database user.
	SSLMode           string // SSL mode for the DB connection.  Usually `disable` or `require`.
	MaxOpenConns      int    // max open connections for the database connection pool.
	MaxIdleConns      int    // max number of idle connections to maintain in the database connection pool.
	ConnectionTimeOut int    // timeout in seconds for trying to connect to the database.
}

type WebServer struct {
	Port       string // the port for the web server to listen for connections on e.g., `8080`
	CNAME      string // the public CNAME for the service.
	Production bool   // set true if the application is running in production mode.
}

type SQS struct {
	AWSRegion, QueueName, AccessKey, SecretKey string
	NumberOfListeners                          int // controls the number of concurrent SQS messages listeners that will be started.
}

// LoadConfig locates and loads the JSON file containing Config information for an appliation.  See the
// Config struct in this package.
// name is the name of the file to try to load e.g., `fits`.  This would usually be the name of the appliction.  LoadConfig
// then looks for a file `name`.json to load Config from.  It tries /etc/sysconfig  first and if it does not
// find a file to load there it falls back the directory the application was started from.
//
// If the config file is succesfully loaded from /etc/sysconfig the application is switched to using syslog.
//
// If a config file can't be found or parsed then log.Fatal is called which will log and error then call os.Exit(1).
//
// Config can be made available early in an applications lifecycle (before init() is called) by using this function
// to init a var e.g.,
//
//    var (
//    	config = cfg.LoadConfig("fits")
//    )
//
// A config JSON file looks like:
// Config holds configuration applications e.g.,
//    {
//    	"DataBase": {
//    		"Host": "localhost",
//    		"Name": "fits",
//    		"User": "fits_r",
//    		"Password": "test",
//    		"MaxOpenConns": 30,
//    		"MaxIdleConns": 20,
//    		"ConnectionTimeOut": 5,
//    		"SSLMode": "disable"
//    	},
//    	"WebServer": {
//    		"Port": "8080",
//                     "CNAME": "my.cool.service"
//    	            "Production": false
//    	},
//           "SQS": {
// 		"AWSRegion": "ap-southeast-2",
// 		"QueueName": "XXX",
// 		"AccessKey": "XXX",
// 		"SecretKey": "XXX",
// 		"NumberOfListeners": 1
// 	},
//    }
func Load(name string) Config {
	log.SetPrefix(name + " ")

	c := name + ".json"

	f, err := ioutil.ReadFile("/etc/sysconfig/" + c)
	if err != nil {
		log.Println("Could not load /etc/sysconfig/" + c + " falling back to local file.")
		f, err = ioutil.ReadFile("./" + c)
		if err != nil {
			log.Println("Can't find any config for " + name)
			log.Fatal(err)
		}
	} else {
		logwriter, err := syslog.New(syslog.LOG_NOTICE, name)
		if err == nil {
			log.Println("** logging to syslog **")
			log.SetOutput(logwriter)
		}
	}

	var d Config
	err = json.Unmarshal(f, &d)
	if err != nil {
		log.Println("Problem parsing config file.")
		log.Fatal(err)
	}

	return d
}

// Postgres returns a connection string that is suitable for use with sql for connecting to a Postgres DB.
func (d *DataBase) Postgres() string {
	return "host=" + d.Host +
		" connect_timeout=" + strconv.Itoa(d.ConnectionTimeOut) +
		" user=" + d.User +
		" password=" + d.Password +
		" dbname=" + d.Name +
		" sslmode=" + d.SSLMode
}
