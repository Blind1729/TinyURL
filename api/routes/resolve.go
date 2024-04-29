package routes

import (
	"TinyURL/api/database"
	"errors"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
)

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")
	r := database.CreateClient(0)
	err := r.Ping(database.CTX).Err()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Unable to connect to redis"})
	}
	defer func(r *redis.Client) {
		err := r.Close()
		if err != nil {
			err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
			if err != nil {
				return
			}
		}
	}(r)
	result, err := r.Get(database.CTX, url).Result()
	if errors.Is(err, redis.Nil) {
		err := c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Short URL not found"})
		if err != nil {
			return err
		}
	} else if err != nil {
		err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Unable to connect to redis"})
		if err != nil {
			return err
		}
	}
	rInr := database.CreateClient(1)
	defer func(rInr *redis.Client) {
		err := rInr.Close()
		if err != nil {
			err := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
			if err != nil {
				return
			}
		}
	}(rInr)

	_ = rInr.Incr(database.CTX, "counter")
	return c.Redirect(result, fiber.StatusMovedPermanently)
}
