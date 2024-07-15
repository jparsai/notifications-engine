package cmd

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/util/misc"
)

func newTemplateCommand(cmdContext *commandContext) *cobra.Command {
	var command = cobra.Command{
		Use:   "template",
		Short: "Notification templates related commands",
		RunE: func(c *cobra.Command, args []string) error {
			return errors.New("select child command")
		},
	}
	command.AddCommand(newTemplateNotifyCommand(cmdContext))
	command.AddCommand(newTemplateGetCommand(cmdContext))

	return &command
}

func newTemplateNotifyCommand(cmdContext *commandContext) *cobra.Command {
	var (
		recipients []string
	)
	var command = cobra.Command{
		Use: "notify NAME RESOURCE_NAME",
		Example: fmt.Sprintf(`
# Trigger notification using in-cluster config map and secret
%s template notify app-sync-succeeded guestbook --recipient slack:my-slack-channel

# Render notification render generated notification in console
%s template notify app-sync-succeeded guestbook
`, cmdContext.cliName, cmdContext.cliName),
		Short: "Generates notification using the specified template and send it to specified recipients",
		RunE: func(c *cobra.Command, args []string) error {
			fmt.Println("######## template_CMD_1")
			cancel := withDebugLogs()
			defer cancel()
			if len(args) < 2 {
				return fmt.Errorf("expected two arguments, got %d", len(args))
			}
			fmt.Println("######## template_CMD_2")
			name := args[0]
			resourceName := args[1]
			api, err := cmdContext.getAPI()
			if err != nil {
				fmt.Println("######## template_CMD_3_error = ", err)
				_, _ = fmt.Fprintf(cmdContext.stderr, "failed to create API: %v\n", err)
				return nil
			}

			fmt.Println("######## template_CMD_4")
			api.AddNotificationService("console", services.NewConsoleService(cmdContext.stdout))

			res, err := cmdContext.loadResource(resourceName)
			if err != nil {
				fmt.Println("######## template_CMD_5_error = ", err)
				_, _ = fmt.Fprintf(cmdContext.stderr, "failed to load resource: %v\n", err)
				return nil
			}

			fmt.Println("######## template_CMD_6")

			for _, recipient := range recipients {
				fmt.Println("######## template_CMD_7_recipient = ", recipient)

				parts := strings.Split(recipient, ":")
				fmt.Println("######## template_CMD_8_parts = ", parts)

				dest := services.Destination{Service: parts[0]}
				fmt.Println("######## template_CMD_9_dest = ", dest)

				if len(parts) > 1 {
					dest.Recipient = parts[1]
				}

				fmt.Println("######## template_CMD_10_res = ", res)
				fmt.Println("######## template_CMD_11_res.Object = ", res.Object)
				fmt.Println("######## template_CMD_12_name = ", name)
				fmt.Println("######## template_CMD_13_dest = ", dest)

				if err := api.Send(res.Object, []string{name}, dest); err != nil {
					fmt.Println("######## template_CMD_14_error = ", err)

					_, _ = fmt.Fprintf(cmdContext.stderr, "#### failed to notify res = %s, name = %s, dest = %s, '%s': %v\n", res, name, dest, recipient, err)
					return nil
				}
				fmt.Println("######## template_CMD_14")
			}
			fmt.Println("######## template_CMD_15")
			return nil
		},
	}
	command.Flags().StringArrayVar(&recipients, "recipient", []string{"console:stdout"}, "List of recipients")

	return &command
}

func newTemplateGetCommand(cmdContext *commandContext) *cobra.Command {
	var (
		output string
	)
	var command = cobra.Command{
		Use: "get",
		Example: fmt.Sprintf(`
# prints all templates
%s template get
# print YAML formatted app-sync-succeeded template definition
%s template get app-sync-succeeded -o=yaml
`, cmdContext.cliName, cmdContext.cliName),
		Short: "Prints information about configured templates",
		RunE: func(c *cobra.Command, args []string) error {
			var name string
			if len(args) == 1 {
				name = args[0]
			}
			items := map[string]services.Notification{}

			api, err := cmdContext.getAPI()
			if err != nil {
				_, _ = fmt.Fprintf(cmdContext.stderr, "failed to get api: %v\n", err)
				return nil
			}
			for n, template := range api.GetConfig().Templates {
				if n == name || name == "" {
					items[n] = template
				}
			}
			switch output {
			case "", "wide":
				w := tabwriter.NewWriter(cmdContext.stdout, 5, 0, 2, ' ', 0)
				_, _ = fmt.Fprintf(w, "NAME\tPREVIEW\n")
				misc.IterateStringKeyMap(items, func(name string) {
					template := items[name]
					_, _ = fmt.Fprintf(w, "%s\t%s\n", name, template.Preview())
				})
				_ = w.Flush()
			case "name":
				misc.IterateStringKeyMap(items, func(name string) {
					_, _ = fmt.Println(name)
				})
			default:
				return misc.PrintFormatted(items, output, cmdContext.stdout)
			}
			return nil
		},
	}
	addOutputFlags(&command, &output)
	return &command
}
