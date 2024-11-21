package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	val "github.com/go-playground/validator/v10/non-standard/validators"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"github.com/joho/godotenv"
)

// Embed a single file
//
//go:embed card.html
var cardFile embed.FS

var validate = validator.New()

type ErrorResponse struct {
	FailedField string
	Tag         string
	Value       string
}

func ValidateStruct(s interface{}) []*ErrorResponse {
	var errors []*ErrorResponse
	err := validate.Struct(s)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element ErrorResponse
			element.FailedField = err.StructNamespace()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}

// How many people
// Where are they from

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS card_swipes (card_number varchar(45) NOT NULL, timestamp TIMESTAMP);")
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	err = validate.RegisterValidation("notblank", val.NotBlank)
	if err != nil {
		panic(err)
	}

	app := fiber.New()
	app.Use(cors.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Welcome to the Card Server!")
	})

	app.Get("/card", func(c *fiber.Ctx) error {
		return filesystem.SendFile(c, http.FS(cardFile), "card.html")
	})

	password := os.Getenv("CARD_PASSWORD")

	app.Get("/send", func(c *fiber.Ctx) error {
		var request struct {
			CardNumber string `json:"card" xml:"card" form:"card" query:"card" validate:"required"`
			Password   string `json:"pw" xml:"pw" form:"pw" query:"pw" validate:"required"`
		}

		if err := c.QueryParser(&request); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		{
			errors := ValidateStruct(request)
			if errors != nil {
				return c.Status(fiber.StatusBadRequest).JSON(errors)
			}
		}

		if request.Password != password {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		conn.Exec(c.Context(), "INSERT INTO card_swipes (card_number, timestamp) VALUES ($1, CURRENT_TIMESTAMP);", request.CardNumber)

		return c.SendStatus(fiber.StatusOK)
	})

	type Data struct {
		UUID      string    `json:"uuid"`
		Timestamp time.Time `json:"timestamp"`
	}

	app.Get("/data", func(c *fiber.Ctx) error {
		var request struct {
			Start    string `json:"start" xml:"start" form:"start" query:"start" validate:"required"`
			End      string `json:"end" xml:"end" form:"end" query:"end" validate:"required"`
			Password string `json:"pw" xml:"pw" form:"pw" query:"pw" validate:"required"`
		}

		if err := c.QueryParser(&request); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		{
			errors := ValidateStruct(request)
			if errors != nil {
				return c.Status(fiber.StatusBadRequest).JSON(errors)
			}
		}

		if request.Password != password {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		start, err := time.Parse(time.RFC3339, request.Start)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Bad start time")
		}

		end, err := time.Parse(time.RFC3339, request.End)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Bad end time")
		}

		rows, err := conn.Query(c.Context(), "SELECT * FROM card_swipes WHERE timestamp BETWEEN $1 and $2;", start, end)
		if err != nil {
			fmt.Println(err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		defer rows.Close()
		var response []Data

		m := make(map[string]string)

		for rows.Next() {
			var card string
			var time time.Time
			err = rows.Scan(&card, &time)
			if err != nil {
				panic(err)
			}

			if m[card] == "" {
				m[card] = uuid.NewString()
			}

			response = append(response, Data{UUID: m[card], Timestamp: time})
		}
		err = rows.Err()
		if err != nil {
			panic(err)
		}

		return c.Status(fiber.StatusOK).JSON(response)
	})

	app.Listen(":3000")
}
