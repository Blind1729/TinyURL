package routes

import (
	"TinyURL/api/database"
	"TinyURL/api/helpers"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"short"`
	Expiry      time.Duration `json:"expiry"`
}

type response struct {
	URL             string        `json:"url"`
	CustomShort     string        `json:"short"`
	Expiry          time.Duration `json:"expiry"`
	XRateRemaining  int           `json:"rate_limit"`
	XRateLimitReset time.Duration `json:"rate_limit_reset"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Can not parse JSON"})
	}

	// Rate limiting
	r2 := database.CreateClient(1)
	defer func(r2 *redis.Client) {
		err := r2.Close()
		if err != nil {
			return
		}
	}(r2)
	result, err := r2.Get(database.CTX, c.IP()).Result()
	if errors.Is(err, redis.Nil) {
		_ = r2.Set(database.CTX, c.IP(), os.Getenv("API_QUOTA"), 30*time.Minute).Err()
	} else {
		valInt, _ := strconv.Atoi(result)
		if valInt <= 0 {
			limit, _ := r2.TTL(database.CTX, c.IP()).Result()
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "Rate limit exceeded", "retry_after": limit / time.Nanosecond / time.Minute})
		}
	}
	// Check if the URL is valid
	if !govalidator.IsURL(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL"})
	}

	// Check for domain error
	if !helpers.RemoveDomainError(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Attempt to exploit the URL service blocked"})
	}

	// Enforce HTTPS, SSL
	body.URL = helpers.EnforceHTTP(body.URL)

	var id string
	body.URL = helpers.EnforceHTTP(body.URL)
	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}
	r := database.CreateClient(0)
	defer func(r *redis.Client) {
		err := r.Close()
		if err != nil {
			err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
			if err != nil {
				return
			}
		}
	}(r)
	val, _ := r.Get(database.CTX, id).Result()
	if val != "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Short URL already exists"})
	}
	if body.Expiry == 0 {
		body.Expiry = 24
	}

	err = r.Set(database.CTX, id, body.URL, body.Expiry*time.Hour).Err()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Unable to connect to redis"})
	}

	resp := response{
		URL:             body.URL,
		CustomShort:     "",
		Expiry:          body.Expiry,
		XRateRemaining:  10,
		XRateLimitReset: 30 * time.Minute,
	}

	r2.Decr(database.CTX, c.IP())

	val, _ = r2.Get(database.CTX, c.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(val)
	ttl, _ := r2.TTL(database.CTX, c.IP()).Result()
	resp.XRateLimitReset = ttl / time.Nanosecond / time.Minute
	resp.CustomShort = os.Getenv("DOMAIN") + "/" + id
	return c.Status(fiber.StatusCreated).JSON(resp)
}
