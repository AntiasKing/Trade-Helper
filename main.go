package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
)

type token struct {
	Token string `json:"token,omitempty"`
}

// Pull is the result of a pull request
type Pull struct {
	MarketPlace string  `json:"marketplace,omitempty"`
	Position    int     `json:"position,omitempty"`
	Value       float64 `json:"value,omitempty"`
}

// Client is the result of a pull request
type Client struct {
	money  float64
	object [nbData]int
}

// Action is the result of a Buy or Sell request
type Action struct {
	Group       string  `json:"group,omitempty"`
	MarketPlace int     `json:"marketplace,omitempty"`
	Action      string  `json:"action,omitempty"`
	Quantity    int     `json:"quantity,omitempty"`
	Price       float64 `json:"price,omitempty"`
}

const (
	nbData = 4
)

var (
	tokenAuth  = jwtauth.New("HS256", []byte("secret"), nil)
	datasName  = [nbData]string{"crypto", "forex", "raw", "stock"}
	paths      = [nbData]string{"./indexes/crypto/", "./indexes/forex/", "./indexes/raw/", "./indexes/stock/"}
	datas      [nbData][]float64
	datasIndex = [nbData]int{0}
	clients    map[string]*Client
)

func writeJSON(w http.ResponseWriter, v interface{}, status int) error {
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func login(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Host + " logged.")
	_, tokenString, _ := tokenAuth.Encode(jwtauth.Claims{"host": r.Host})
	clients[tokenString] = &Client{
		money:  10000,
		object: [nbData]int{0},
	}
	writeJSON(w, &token{Token: tokenString}, http.StatusOK)
}

func resetToken(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Host + " reset tokken.")
	_, tokenString, _ := tokenAuth.Encode(jwtauth.Claims{"host": r.Host})
	writeJSON(w, &token{Token: tokenString}, http.StatusOK)
}

func pull(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Host + " pulled.")
	marketID, err := strconv.Atoi(chi.URLParam(r, "marketId"))
	if err != nil {
		fmt.Println(err.Error())
		writeJSON(w, err.Error(), http.StatusBadRequest)
		return
	}
	count, err := strconv.Atoi(chi.URLParam(r, "count"))
	if err != nil {
		count = 0
	}
	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil {
		index = datasIndex[marketID]
	}
	if marketID < 0 || count < 0 || index < 0 || marketID >= nbData || index > datasIndex[marketID] || index-count < 0 {
		writeJSON(w, "Bad Request.", http.StatusBadRequest)
		return
	}
	var result []Pull
	for i := index - count; i <= index; i++ {
		result = append(result, Pull{
			MarketPlace: datasName[marketID],
			Position:    i,
			Value:       datas[marketID][i],
		})
	}
	writeJSON(w, result, http.StatusOK)
}

func action(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Host + " action.")
	tok := jwtauth.TokenFromHeader(r)
	action := chi.URLParam(r, "action")
	marketID, err := strconv.Atoi(chi.URLParam(r, "market"))
	if err != nil {
		fmt.Println(err.Error())
		writeJSON(w, err.Error(), http.StatusBadRequest)
		return
	}
	quantity, err := strconv.Atoi(chi.URLParam(r, "quantity"))
	if err != nil {
		fmt.Println(err.Error())
		writeJSON(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch action {
	case "buy":
		if datas[marketID][datasIndex[marketID]]*float64(quantity) > clients[tok].money {
			writeJSON(w, "Not enought objects.", http.StatusBadRequest)
			return
		}
		clients[tok].money -= datas[marketID][datasIndex[marketID]] * float64(quantity)
		clients[tok].object[marketID] += quantity

	case "sell":
		if clients[tok].object[marketID] < quantity {
			writeJSON(w, "Not enought money.", http.StatusBadRequest)
			return
		}
		clients[tok].money += datas[marketID][datasIndex[marketID]] * float64(quantity)
		clients[tok].object[marketID] -= quantity
	default:
		writeJSON(w, action+" unknow.", http.StatusBadRequest)
		return
	}
	writeJSON(w, &Action{
		Group:       tok,
		MarketPlace: marketID,
		Action:      action,
		Quantity:    quantity,
		Price:       datas[marketID][datasIndex[marketID]] * float64(quantity)},
		http.StatusOK)
}

func router() (r *chi.Mux) {
	r = chi.NewRouter()
	r.Get("/login", login)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))

		r.Use(jwtauth.Authenticator)

		r.Get("/reset_token", resetToken)

		r.Get("/group/pull/marketplace_id={marketId}", pull)
		r.Get("/group/pull/marketplace_id={marketId}&count={count}", pull)
		r.Get("/group/pull/marketplace_id={marketId}&count={count}&index={index}", pull)

		r.Get("/group/{action}/marketplace_id={market}&quantity={quantity}", action)
	})
	return
}

func getDatas(path string, slice []float64) ([]float64, error) {
	content, err := ioutil.ReadDir(path)
	if err != nil {
		return slice, err
	}
	for _, file := range content {
		s, err := getData(path+file.Name(), slice)
		slice = append(slice, s...)
		if err != nil {
			return slice, err
		}

	}
	return slice, nil
}

func getData(file string, slice []float64) ([]float64, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(bufio.NewReader(fd))
	for s.Scan() {
		f, err := strconv.ParseFloat(s.Text(), 64)
		if err != nil {
			return nil, err
		}
		slice = append(slice, f)
	}
	return slice, nil
}

func printClient() {
	for {
		for i, x := range datasIndex {
			if x+1 >= len(datas[i]) {
				continue
			}
			datasIndex[i]++
		}
		time.Sleep(5 * time.Second)
	}
}

func main() {
	clients = make(map[string]*Client)
	for x, path := range paths {
		s, err := getDatas(path, datas[x])
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(84)
		}
		datas[x] = s
	}
	go printClient()
	r := router()
	fmt.Println(http.ListenAndServe(":8484", r).Error())
}
