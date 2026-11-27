package discord

import (
	"crypto/rand"
	"encoding/hex"
	mrand "math/rand/v2"
	"strconv"
	"time"
)

// DiscordEpoch is the first millisecond of 2015, the epoch of Discord snowflakes.
const DiscordEpoch = 1420070400000

// UtcNow returns the current time in UTC.
func UtcNow() time.Time {
	return time.Now().UTC()
}

// TimeSnowflake returns the lowest (or, with high, the highest) snowflake
// that could have been created at dt. Useful for before/after pagination.
func TimeSnowflake(dt time.Time, high bool) int64 {
	discordMillis := dt.UnixMilli() - DiscordEpoch
	if high {
		return (discordMillis << 22) + (1<<22 - 1)
	}
	return discordMillis << 22
}

// SnowflakeTime returns the creation time of a Discord ID.
func SnowflakeTime(id string) (time.Time, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli((n >> 22) + DiscordEpoch).UTC(), nil
}

// GenerateNonce returns a snowflake-like nonce for messages and interactions.
func GenerateNonce() string {
	return strconv.FormatInt(TimeSnowflake(UtcNow(), false)|mrand.Int64N(1<<22), 10)
}

// GenerateSessionID returns a random 32 character hex session ID.
func GenerateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(mrand.IntN(256))
		}
	}
	return hex.EncodeToString(b)
}

// GenerateSuperProperties returns the base64 encoded X-Super-Properties header.
//
// Deprecated: use gateway.SuperPropertiesHeader.
func GenerateSuperProperties(gateway *Gateway) string {
	return gateway.SuperPropertiesHeader()
}
