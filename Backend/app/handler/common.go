package handler

import (
	"../model"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

/*
	Common methods shared amongst the different models
*/

func NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteData("Resource Not Found", http.StatusNotFound, w)
	})
}

/*
Note: Need to add more authentication checks later (This is temporary)
 */
func IsValidJWT(w http.ResponseWriter, r *http.Request) bool {
	if token := r.Header["Token"]; token != nil {
		if t, err := jwt.Parse(token[0], kf); err == nil {
			if t.Valid {
				return true
			}
		}
	}
	WriteData("Unauthorized", http.StatusUnauthorized, w)
	return false
}

func kf(token *jwt.Token) (interface{}, error) {
	// Verifying that the signing method is the same before continuing any further
	if _, accepted := token.Method.(*jwt.SigningMethodHMAC); !accepted {
		return nil, fmt.Errorf("an error occured")
	}
	// Note: This must be changed to an env variable later
	return []byte("2ofClubs"), nil
}

func GenerateJWT(subject string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	sysTime := time.Now()
	claims["iat"] = sysTime
	claims["exp"] = sysTime.Add(time.Minute * 5).Unix()
	claims["sub"] = subject
	// Note: This must be changed to an env variable later
	tokenString, err := token.SignedString([]byte("2ofClubs"))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func GenerateSaltedPass(password string) (string, bool) {
	saltedHashPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", false
	}
	return string(saltedHashPass), true
}
func RecordExists(db *gorm.DB, column string, val string, t interface{}) bool {
	return !db.Where(column+"= ?", val).First(t).RecordNotFound()
}

func ExtractPersonInfo(r *http.Request) model.Person {
	decoder := json.NewDecoder(r.Body)
	p := model.NewPerson()
	decoder.Decode(&p)
	return p
}

func ParseJSON(response interface{}) string {
	data, err := json.MarshalIndent(response, "", "\t")
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(data)
}

func WriteData(data string, code int, w http.ResponseWriter) int {
	w.WriteHeader(code)
	n, err := fmt.Fprint(w, data)
	if err != nil {
		return -1
	}
	return n
}
