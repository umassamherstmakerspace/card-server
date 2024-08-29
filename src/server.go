package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-playground/validator/v10"
	val "github.com/go-playground/validator/v10/non-standard/validators"

	"github.com/jackc/pgx/v5"

	"github.com/gofiber/fiber/v2"

	"github.com/joho/godotenv"
)

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

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Welcome to the Card Server!")
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

	app.Listen(":3000")
}
