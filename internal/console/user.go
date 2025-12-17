package console

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"go_framework/internal/db"
	"go_framework/internal/uuid"
)

var userEmail string
var userPassword string
var userRole string
var userID string
var listLimit int
var userConfirm bool

func init() {
	// Register flags on the `create` subcommand so they work when
	// invoked as: `console user create --email ...`
	userCreateCmd.Flags().StringVar(&userEmail, "email", "", "user email")
	userCreateCmd.Flags().StringVar(&userPassword, "password", "", "user password")
	userCreateCmd.Flags().StringVar(&userRole, "role", "admin", "user role")
	userCreateCmd.MarkFlagRequired("email")
	// Add the parent `user` command to the root (subcommands are added later)
	rootCmd.AddCommand(userCmd)
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User management commands",
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a user",
	Run: func(cmd *cobra.Command, args []string) {
		if userPassword == "" {
			fmt.Print("Password: ")
			fmt.Scanln(&userPassword)
		}
		if err := createUser(userEmail, userPassword, userRole); err != nil {
			log.Fatalf("failed to create user: %v", err)
		}
		fmt.Println("user created")
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users",
	Run: func(cmd *cobra.Command, args []string) {
		if err := listUsers(listLimit); err != nil {
			log.Fatalf("failed to list users: %v", err)
		}
	},
}

var userUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a user (by --id or --email)",
	Run: func(cmd *cobra.Command, args []string) {
		if userID == "" && userEmail == "" {
			log.Fatalf("either --id or --email is required to identify the user")
		}
		if userPassword == "" && userRole == "" {
			log.Fatalf("nothing to update: provide --password and/or --role")
		}
		if err := updateUser(userID, userEmail, userPassword, userRole); err != nil {
			log.Fatalf("failed to update user: %v", err)
		}
		fmt.Println("user updated")
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a user (by --id or --email)",
	Run: func(cmd *cobra.Command, args []string) {
		if userID == "" && userEmail == "" {
			log.Fatalf("either --id or --email is required to identify the user")
		}
		if !userConfirm {
			var ans string
			fmt.Printf("Are you sure you want to delete user (id=%s email=%s)? (y/N): ", userID, userEmail)
			fmt.Scanln(&ans)
			if ans != "y" && ans != "Y" {
				fmt.Println("aborted")
				return
			}
		}
		if err := deleteUser(userID, userEmail); err != nil {
			log.Fatalf("failed to delete user: %v", err)
		}
		fmt.Println("user deleted")
	},
}

var userGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a user (by --id or --email)",
	Run: func(cmd *cobra.Command, args []string) {
		if userID == "" && userEmail == "" {
			log.Fatalf("either --id or --email is required to identify the user")
		}
		if err := getUser(userID, userEmail); err != nil {
			log.Fatalf("failed to get user: %v", err)
		}
	},
}

func createUser(email, password, role string) error {
	// generate a UUIDv7 id in the application and insert it explicitly
	id, err := uuid.New()
	if err != nil {
		return err
	}
	dbConn, err := db.GetDB()
	if err != nil {
		return err
	}
	defer dbConn.Close()

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Insert into users table; create migration ensures table exists
	q := `INSERT INTO users (id, email, password_hash, role, created_at, updated_at) VALUES ($1, $2, $3, $4, now(), now())`
	_, err = dbConn.Exec(q, id, email, string(hashed), role)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no rows")
		}
		return err
	}
	return nil
}

func listUsers(limit int) error {
	dbConn, err := db.GetDB()
	if err != nil {
		return err
	}
	defer dbConn.Close()

	var rows *sql.Rows
	if limit > 0 {
		rows, err = dbConn.Query(`SELECT id, email, role, created_at FROM users ORDER BY created_at DESC LIMIT $1`, limit)
	} else {
		rows, err = dbConn.Query(`SELECT id, email, role, created_at FROM users ORDER BY created_at DESC`)
	}
	if err != nil {
		return err
	}
	defer rows.Close()

	// Pretty-print as a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tEMAIL\tROLE\tCREATED_AT")
	for rows.Next() {
		var id, email, role string
		var created sql.NullString
		if err := rows.Scan(&id, &email, &role, &created); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, email, role, created.String)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return w.Flush()
}

func updateUser(id, email, password, role string) error {
	dbConn, err := db.GetDB()
	if err != nil {
		return err
	}
	defer dbConn.Close()

	// Build update set dynamically
	sets := []string{}
	args := []interface{}{}
	i := 1
	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		sets = append(sets, fmt.Sprintf("password_hash = $%d", i))
		args = append(args, string(hashed))
		i++
	}
	if role != "" {
		sets = append(sets, fmt.Sprintf("role = $%d", i))
		args = append(args, role)
		i++
	}
	if len(sets) == 0 {
		return fmt.Errorf("nothing to update")
	}
	sets = append(sets, fmt.Sprintf("updated_at = now()"))
	// Identify by id or email
	var where string
	if id != "" {
		where = fmt.Sprintf("id = $%d", i)
		args = append(args, id)
	} else {
		where = fmt.Sprintf("email = $%d", i)
		args = append(args, email)
	}

	q := fmt.Sprintf("UPDATE users SET %s WHERE %s", strings.Join(sets, ", "), where)
	res, err := dbConn.Exec(q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func deleteUser(id, email string) error {
	dbConn, err := db.GetDB()
	if err != nil {
		return err
	}
	defer dbConn.Close()

	var res sql.Result
	if id != "" {
		res, err = dbConn.Exec(`DELETE FROM users WHERE id = $1`, id)
	} else {
		res, err = dbConn.Exec(`DELETE FROM users WHERE email = $1`, email)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func getUser(id, email string) error {
	dbConn, err := db.GetDB()
	if err != nil {
		return err
	}
	defer dbConn.Close()

	var row *sql.Row
	if id != "" {
		row = dbConn.QueryRow(`SELECT id, email, role, metadata, created_at, updated_at, deleted_at FROM users WHERE id = $1`, id)
	} else {
		row = dbConn.QueryRow(`SELECT id, email, role, metadata, created_at, updated_at, deleted_at FROM users WHERE email = $1`, email)
	}

	var uid, uemail, urole string
	var metadata sql.NullString
	var created, updated time.Time
	var deleted sql.NullTime

	if err := row.Scan(&uid, &uemail, &urole, &metadata, &created, &updated, &deleted); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user not found")
		}
		return err
	}

	// Print key-value output for single user
	fmt.Printf("ID: %s\n", uid)
	fmt.Printf("Email: %s\n", uemail)
	fmt.Printf("Role: %s\n", urole)
	if metadata.Valid && metadata.String != "{}" {
		fmt.Printf("Metadata: %s\n", metadata.String)
	}
	fmt.Printf("Created At: %s\n", created.Format(time.RFC3339))
	fmt.Printf("Updated At: %s\n", updated.Format(time.RFC3339))
	if deleted.Valid {
		fmt.Printf("Deleted At: %s\n", deleted.Time.Format(time.RFC3339))
	}
	return nil
}

func init() {
	userCmd.AddCommand(userCreateCmd)
	// List users
	userListCmd.Flags().IntVar(&listLimit, "limit", 0, "limit number of results (0=no limit)")
	userCmd.AddCommand(userListCmd)

	// Update user
	userUpdateCmd.Flags().StringVar(&userID, "id", "", "user id (uuid)")
	userUpdateCmd.Flags().StringVar(&userEmail, "email", "", "user email")
	userUpdateCmd.Flags().StringVar(&userPassword, "password", "", "new password")
	userUpdateCmd.Flags().StringVar(&userRole, "role", "", "new role")
	userCmd.AddCommand(userUpdateCmd)

	// Delete user
	userDeleteCmd.Flags().StringVar(&userID, "id", "", "user id (uuid)")
	userDeleteCmd.Flags().StringVar(&userEmail, "email", "", "user email")
	userDeleteCmd.Flags().BoolVar(&userConfirm, "yes", false, "confirm deletion without prompt")
	userCmd.AddCommand(userDeleteCmd)

	// Get user
	userGetCmd.Flags().StringVar(&userID, "id", "", "user id (uuid)")
	userGetCmd.Flags().StringVar(&userEmail, "email", "", "user email")
	userCmd.AddCommand(userGetCmd)
}
