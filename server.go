package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//Robot must be exported so that json package can access it to encode
// all strings because they're going to be serialized to JSON anyways
type Robot struct {
	Name      string // the name of the robot
	User      string // the user logged in to the robot
	IP        string // the ip address of the robot
	X         string // x coordinate
	Y         string // y coordinate
	Alive     string // if the robot is currently active
	LastAlive string // epoch time since robot last pinged server
}

func (bot Robot) String() string {
	ret := ""
	ret += bot.Name + "\n"
	ret += "\t" + "User: " + bot.User + "\n"
	ret += "\t" + "IP: " + bot.IP + "\n"
	ret += "\t" + "Coordinates: (" + bot.X + ", " + bot.Y + ")\n"
	ret += "\t" + "Alive: " + bot.Alive + "\n"
	ret += "\t" + "Time Last Alive: " + bot.LastAlive + "\n"
	return ret
}

var robots []Robot

// command line args
var portNumber int
var file string
var debug bool

var saveTicker = time.NewTicker(20 * time.Second) // controls time between saves

var aliveTimeout = 10 // (in seconds) if a robot isn't heard from in this time, it's not alive

/////////////
// ROUTING //
/////////////

// converts a string-returning function to a function that writes to
func serveBasicHTML(f func() string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, f())
	}
}

//define functions for server endpoints
////    /update           called by robots with their info
////		/text							serves all robot info
////    /json             serves all robot info as json
////    /hosts        		legacy support, serves robotname:IP
////    /hostsjson        legacy support, serves json of robotname:IP
////    /hostsalivejson   legacy support, serves json of robotname:IP of active robots

func update(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	pdebug("IP of request: " + r.RemoteAddr)
	fmt.Fprintf(w, updateRobot(query, r.RemoteAddr))
	save()
}

func jsonFull() string {
	bytes, err := json.Marshal(robots)
	checkErr(err, "couldn't jsonify")
	return string(bytes)
}

func hosts() string {
	ret := ""
	for _, bot := range robots {
		ret += bot.Name + "\t\t"
		ret += bot.IP + "\n"
	}
	return ret
}

func hostsJSON() string {
	ret := "{"
	for _, bot := range robots {
		ret += "\"" + bot.Name + "\":"
		ret += "\"" + bot.IP + "\","
	}
	return strings.TrimSuffix(ret, ",") + "}"
}

func hostsAliveJSON() string {
	ret := "{"
	for _, bot := range robots {
		if bot.Alive == "true" {
			ret += "\"" + bot.Name + "\":"
			ret += "\"" + bot.IP + "\","
		}
	}
	return strings.TrimSuffix(ret, ",") + "}"
}

func textFull() string {
	ret := ""
	for _, bot := range robots {
		ret += bot.String()
	}
	return ret
}

///////////////////////
// utility functions //
///////////////////////

func pdebug(message string) {
	if debug {
		fmt.Println(message)
	}
}

//error helper, prints error message if there's an error
func checkErr(err error, message string) {
	if err != nil {
		log.Printf("Error: %s\n", message)
		log.Println(err)
	}
}

/////////
// I/O //
/////////

func save() {
	pdebug("saving to local file")
	bytes, err := json.Marshal(robots)
	checkErr(err, "couldn't marshal the DNS")
	err = ioutil.WriteFile(file, bytes, 0644)
	checkErr(err, "couldn't write to "+file)
}

func load() {
	pdebug("reading from " + file)
	bytes, err := ioutil.ReadFile(file)
	checkErr(err, "couldn't read from "+file)
	err = json.Unmarshal(bytes, &robots)
	checkErr(err, "couldn't unmarshal data read from "+file)
}

func addRobot(query url.Values, addr string) {
	bot := Robot{
		Name:      query.Get("name"),
		User:      query.Get("user"),
		IP:        addr,
		X:         query.Get("x"),
		Y:         query.Get("y"),
		Alive:     "true",
		LastAlive: strconv.FormatInt(time.Now().Unix(), 10)}
	pdebug("Adding new robot: " + string(bot.Name))
	robots = append(robots, bot)
}

func updateRobot(query url.Values, addr string) string {
	for i, bot := range robots {
		if bot.Name == query.Get("name") {
			robots[i].User = query.Get("user")
			robots[i].IP = addr
			robots[i].X = query.Get("x")
			robots[i].Y = query.Get("y")

			oldTime, _ := strconv.ParseInt(robots[i].LastAlive, 10, 64)
			newTime := time.Now().Unix()
			robots[i].LastAlive = strconv.FormatBool(newTime-oldTime < aliveTimeout)
			robots[i].LastAlive = strconv.FormatInt(newTime, 10)

			pdebug("Updated " + query.Get("name"))
			return "updated " + query.Get("name")
		}
	}
	addRobot(query, addr) // if it's not in robots, add it
	return "Added " + query.Get("name")
}

func main() {
	flag.BoolVar(&debug, "debug", false, "print debug info")
	flag.IntVar(&portNumber, "port", 7978, "port number")
	flag.StringVar(&file, "file", ".robot_statuses", "file to save robot statuses")
	flag.Parse()

	//load from local files/database
	load()

	//bind/start server
	http.HandleFunc("/update", update)
	http.HandleFunc("/json", serveBasicHTML(jsonFull))
	http.HandleFunc("/text", serveBasicHTML(textFull))
	http.HandleFunc("/hosts", serveBasicHTML(hosts))
	http.HandleFunc("/hostsjson", serveBasicHTML(hostsJSON))
	http.HandleFunc("/hostsalivejson", serveBasicHTML(hostsAliveJSON))

	fmt.Println("starting server on port " + strconv.Itoa(portNumber))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(portNumber), nil))
}
