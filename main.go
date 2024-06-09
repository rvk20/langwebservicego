package main

import (
	"database/sql"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// Ustawienie headerów
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Word reprezentuje słowo w bazie danych.
type Word struct {
	ID          int    // Unikalny identyfikator słowa.
	ForeignWord string // Słowo w języku obcym.
	PolishWord  string // Tłumaczenie słowa na język polski.
	Count       int    // Licznik poprawnych odpowiedzi.
}

// RandomWord reprezentuje losowo wybrane słowo.
type RandomWord struct {
	ID          int    // Unikalny identyfikator słowa.
	PolishWord  string // Tłumaczenie słowa na język polski.
	ForeignWord string // Słowo w języku obcym.
}

// AnswerWord reprezentuje odpowiedź użytkownika.
type AnswerWord struct {
	ID          string `json:"id" binding:"required"`           // Unikalny identyfikator słowa.
	ForeignWord string `json:"foreign_word" binding:"required"` // Słowo w języku obcym podane przez użytkownika.
	PolishWord  string `json:"polish_word" binding:"required"`  // Tłumaczenie słowa na język polski podane przez użytkownika.
}

// ResponseWord reprezentuje odpowiedź serwera.
type ResponseWord struct {
	ForeignWord     string // Słowo w języku obcym.
	PolishWord      string // Tłumaczenie słowa na język polski.
	IsCorrectAnswer bool   // Czy odpowiedź użytkownika była poprawna.
}

// openDBConnection otwiera połączenie z bazą danych MySQL.
func openDBConnection() (*sql.DB, error) {
	// Konfiguracja połączenia z bazą danych.
	db, err := sql.Open("mysql", "root:@tcp(localhost:3306)/words_base")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// getRandomWord pobiera losowe słowo z bazy danych.
func getRandomWord(db *sql.DB) (RandomWord, error) {
	// Wykonanie zapytania SQL do bazy danych.
	rows, err := db.Query("SELECT * FROM words")
	if err != nil {
		return RandomWord{}, err
	}
	defer rows.Close()

	// Przetwarzanie wyników zapytania.
	words := []Word{}
	for rows.Next() {
		var id int
		var foreignWord string
		var polishWord string
		var count int
		if err := rows.Scan(&id, &foreignWord, &polishWord, &count); err != nil {
			return RandomWord{}, err
		}
		words = append(words, Word{ID: id, ForeignWord: foreignWord, PolishWord: polishWord})
	}

	// Wybór losowego słowa.
	wordsLength := len(words)
	randomElement := rand.Intn(wordsLength)
	randomWord := RandomWord{ID: words[randomElement].ID, PolishWord: words[randomElement].PolishWord, ForeignWord: words[randomElement].ForeignWord}

	return randomWord, nil
}

// getWordById pobiera słowo z bazy danych na podstawie ID.
func getWordById(db *sql.DB, id string) (Word, error) {
	var word Word
	// Wykonanie zapytania SQL z parametrem.
	err := db.QueryRow("SELECT * FROM words WHERE id = ?", id).
		Scan(&word.ID, &word.ForeignWord, &word.PolishWord, &word.Count)
	if err != nil {
		return Word{}, err
	}

	return word, nil
}

// compareWords porównuje dwa słowa i zwraca true jeśli są identyczne.
func compareWords(answer string, correctWord string) bool {
	return answer == correctWord
}

// updateWordAfterAnswer aktualizuje licznik słowa w bazie danych po odpowiedzi użytkownika.
func updateWordAfterAnswer(db *sql.DB, isAnswerCorrect bool, word Word) {
	if isAnswerCorrect {
		if word.Count > 6 {
			// Dodanie rekordu do nauczonych słów.
			_, err := db.Exec("INSERT INTO learned_words(foreign_word, polish_word) VALUES (?, ?)",
				word.ForeignWord, word.PolishWord)
			if err != nil {
				panic(err.Error())
			}

			// Usunięcie rekordu, po 8 poprawnych odpowiedziach.
			_, err2 := db.Exec("DELETE FROM words WHERE id = ?",
				word.ID)
			if err2 != nil {
				panic(err2.Error())
			}
		} else {
			// Inkrementacja licznika poprawnych odpowiedzi.
			_, err := db.Exec("UPDATE words SET count = ? WHERE id = ?",
				word.Count+1, word.ID)
			if err != nil {
				panic(err.Error())
			}
		}

	} else {
		// Resetowanie licznika poprawnych odpowiedzi.
		addWord(db, word)
	}
}

// Dodanie nowego słowa.
func addWord(db *sql.DB, word Word) {
	// Dodanie nowego słowa
	_, err := db.Exec("INSERT INTO words (foreign_word, polish_word, count) VALUES (?, ?, ?)",
		word.ForeignWord, word.PolishWord, 0)
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	// Inicjalizacja serwera HTTP z frameworkiem Gin.
	r := gin.Default()

	// Użyj middleware CORS przed innymi endpointami
	r.Use(CORSMiddleware())

	// Otwarcie połączenia z bazą danych.
	db, err := openDBConnection()
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	// Definicja endpointu GET do pobierania losowego słowa.
	r.GET("/get_random_word", func(c *gin.Context) {
		randomWord, err := getRandomWord(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, randomWord)
	})

	// Definicja endpointu POST do przesyłania odpowiedzi użytkownika.
	r.POST("/post_answer", func(c *gin.Context) {
		var answer AnswerWord
		c.BindJSON(&answer)
		word, err := getWordById(db, answer.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		isAnswerCorrect := compareWords(answer.ForeignWord, word.ForeignWord)
		updateWordAfterAnswer(db, isAnswerCorrect, word)

		c.JSON(http.StatusOK, ResponseWord{ForeignWord: word.ForeignWord, PolishWord: word.PolishWord, IsCorrectAnswer: isAnswerCorrect})
	})

	// Definicja endpointu POST do przesyłania polskiej odpowiedzi użytkownika.
	r.POST("/post_answer_polish", func(c *gin.Context) {
		var answer AnswerWord
		c.BindJSON(&answer)
		word, err := getWordById(db, answer.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		isAnswerCorrect := compareWords(answer.PolishWord, word.PolishWord)
		updateWordAfterAnswer(db, isAnswerCorrect, word)

		c.JSON(http.StatusOK, ResponseWord{ForeignWord: word.ForeignWord, PolishWord: word.PolishWord, IsCorrectAnswer: isAnswerCorrect})
	})

	// Definicja endpointu POST do dodawania nowych słów.
	r.POST("/add_word", func(c *gin.Context) {
		var word Word
		c.BindJSON(&word)
		addWord(db, word)
	})

	// Uruchomienie serwera na porcie 8080.
	r.Run(":8080")
}
