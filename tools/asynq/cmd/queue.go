// Copyright 2020 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/go-redis/redis/v7"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynq/internal/rdb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const separator = "================================================="

func init() {
	rootCmd.AddCommand(queueCmd)
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queueInspectCmd)
	queueCmd.AddCommand(queueHistoryCmd)
	queueHistoryCmd.Flags().IntP("days", "x", 10, "show data from last x days")

	queueCmd.AddCommand(queuePauseCmd)
	queueCmd.AddCommand(queueUnpauseCmd)
	queueCmd.AddCommand(queueRemoveCmd)
	queueRemoveCmd.Flags().BoolP("force", "f", false, "remove the queue regardless of its size")
}

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage queues",
}

var queueListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List queues",
	// TODO: Use RunE instead?
	Run: queueList,
}

var queueInspectCmd = &cobra.Command{
	Use:   "inspect QUEUE [QUEUE...]",
	Short: "Display detailed information on one or more queues",
	Args:  cobra.MinimumNArgs(1),
	// TODO: Use RunE instead?
	Run: queueInspect,
}

var queueHistoryCmd = &cobra.Command{
	Use:   "history QUEUE [QUEUE...]",
	Short: "Display historical aggregate data from one or more queues",
	Args:  cobra.MinimumNArgs(1),
	Run:   queueHistory,
}

var queuePauseCmd = &cobra.Command{
	Use:   "pause QUEUE [QUEUE...]",
	Short: "Pause one or more queues",
	Args:  cobra.MinimumNArgs(1),
	Run:   queuePause,
}

var queueUnpauseCmd = &cobra.Command{
	Use:   "unpause QUEUE [QUEUE...]",
	Short: "Unpause one or more queues",
	Args:  cobra.MinimumNArgs(1),
	Run:   queueUnpause,
}

var queueRemoveCmd = &cobra.Command{
	Use:   "rm QUEUE [QUEUE...]",
	Short: "Remove one or more queues",
	Args:  cobra.MinimumNArgs(1),
	Run:   queueRemove,
}

func queueList(cmd *cobra.Command, args []string) {
	i := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     viper.GetString("uri"),
		DB:       viper.GetInt("db"),
		Password: viper.GetString("password"),
	})
	queues, err := i.Queues()
	if err != nil {
		fmt.Printf("error: Could not fetch list of queues: %v\n", err)
		os.Exit(1)
	}
	for _, qname := range queues {
		fmt.Println(qname)
	}
}

func queueInspect(cmd *cobra.Command, args []string) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     viper.GetString("uri"),
		DB:       viper.GetInt("db"),
		Password: viper.GetString("password"),
	})
	for i, qname := range args {
		if i > 0 {
			fmt.Printf("\n%s\n", separator)
		}
		fmt.Printf("\nQueue: %s\n\n", qname)
		stats, err := inspector.CurrentStats(qname)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		printQueueStats(stats)
	}
}

func printQueueStats(s *asynq.QueueStats) {
	fmt.Printf("Size: %d\n", s.Size)
	fmt.Printf("Paused: %t\n\n", s.Paused)
	fmt.Println("Task Breakdown:")
	printTable(
		[]string{"InProgress", "Enqueued", "Scheduled", "Retry", "Dead"},
		func(w io.Writer, tmpl string) {
			fmt.Fprintf(w, tmpl, s.InProgress, s.Enqueued, s.Scheduled, s.Retry, s.Dead)
		},
	)
	fmt.Println()
	fmt.Printf("%s Stats:\n", s.Timestamp.UTC().Format("2006-01-02"))
	printTable(
		[]string{"Processed", "Failed", "Error Rate"},
		func(w io.Writer, tmpl string) {
			var errRate string
			if s.Processed == 0 {
				errRate = "N/A"
			} else {
				errRate = fmt.Sprintf("%.2f%%", float64(s.Failed)/float64(s.Processed)*100)
			}
			fmt.Fprintf(w, tmpl, s.Processed, s.Failed, errRate)
		},
	)
}

func queueHistory(cmd *cobra.Command, args []string) {
	days, err := cmd.Flags().GetInt("days")
	if err != nil {
		fmt.Printf("error: Internal error: %v\n", err)
		os.Exit(1)
	}
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     viper.GetString("uri"),
		DB:       viper.GetInt("db"),
		Password: viper.GetString("password"),
	})
	for i, qname := range args {
		if i > 0 {
			fmt.Printf("\n%s\n", separator)
		}
		fmt.Printf("\nQueue: %s\n\n", qname)
		stats, err := inspector.History(qname, days)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		printDailyStats(stats)
	}
}

func printDailyStats(stats []*asynq.DailyStats) {
	printTable(
		[]string{"Date (UTC)", "Processed", "Failed", "Error Rate"},
		func(w io.Writer, tmpl string) {
			for _, s := range stats {
				var errRate string
				if s.Processed == 0 {
					errRate = "N/A"
				} else {
					errRate = fmt.Sprintf("%.2f%%", float64(s.Failed)/float64(s.Processed)*100)
				}
				fmt.Fprintf(w, tmpl, s.Date.Format("2006-01-02"), s.Processed, s.Failed, errRate)
			}
		},
	)
}

func queuePause(cmd *cobra.Command, args []string) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     viper.GetString("uri"),
		DB:       viper.GetInt("db"),
		Password: viper.GetString("password"),
	})
	for _, qname := range args {
		err := inspector.PauseQueue(qname)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("Successfully paused queue %q\n", qname)
	}
}

func queueUnpause(cmd *cobra.Command, args []string) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     viper.GetString("uri"),
		DB:       viper.GetInt("db"),
		Password: viper.GetString("password"),
	})
	for _, qname := range args {
		err := inspector.UnpauseQueue(qname)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("Successfully unpaused queue %q\n", qname)
	}
}

func queueRemove(cmd *cobra.Command, args []string) {
	// TODO: Use inspector once RemoveQueue become public API.
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		fmt.Printf("error: Internal error: %v\n", err)
		os.Exit(1)
	}

	c := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("uri"),
		DB:       viper.GetInt("db"),
		Password: viper.GetString("password"),
	})
	r := rdb.NewRDB(c)
	for _, qname := range args {
		err = r.RemoveQueue(qname, force)
		if err != nil {
			if _, ok := err.(*rdb.ErrQueueNotEmpty); ok {
				fmt.Printf("error: %v\nIf you are sure you want to delete it, run 'asynq queue rm --force %s'\n", err, qname)
				continue
			}
			fmt.Printf("error: %v\n", err)
			continue
		}
		fmt.Printf("Successfully removed queue %q\n", qname)
	}
}
