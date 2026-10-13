package libc

import (
	"fmt"
	"time"
)

// Ctime is ctime(3): asctime's "%.3s %.3s%3d %.2d:%.2d:%.2d %d\n".
func Ctime(t time.Time) string {
	return fmt.Sprintf("%.3s %.3s%3d %02d:%02d:%02d %d\n",
		t.Weekday().String(), t.Month().String(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Year())
}
