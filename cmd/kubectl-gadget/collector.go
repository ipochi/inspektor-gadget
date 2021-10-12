// Copyright 2019-2021 The Inspektor Gadget authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"

	"github.com/kinvolk/inspektor-gadget/cmd/kubectl-gadget/utils"
	gadgetv1alpha1 "github.com/kinvolk/inspektor-gadget/pkg/api/v1alpha1"
	socketcollectortypes "github.com/kinvolk/inspektor-gadget/pkg/gadgets/socket-collector/types"
)

var processCollectorCmd = &cobra.Command{
	Use:   "process-collector",
	Short: "Gather information about running processes",
	Run:   processCollectorCmdRun,
}

var socketCollectorCmd = &cobra.Command{
	Use:   "socket-collector",
	Short: "Gather information about network sockets",
	Run:   socketCollectorCmdRun,
}

var (
	processCollectorParamThreads bool
)

func init() {
	commands := []*cobra.Command{
		processCollectorCmd,
		socketCollectorCmd,
	}

	// Add common flags for all collector gadgets
	for _, command := range commands {
		rootCmd.AddCommand(command)
		utils.AddCommonFlags(command, &params)
	}

	// Add specific flags
	processCollectorCmd.PersistentFlags().BoolVarP(
		&processCollectorParamThreads,
		"threads",
		"t",
		false,
		"Show all threads",
	)
}

func processCollectorCmdRun(cmd *cobra.Command, args []string) {
	callback := func(contextLogger *log.Entry, nodes *corev1.NodeList, results *gadgetv1alpha1.TraceList) {
		// Display results
		type Process struct {
			Tgid                int    `json:"tgid,omitempty"`
			Pid                 int    `json:"pid,omitempty"`
			Comm                string `json:"comm,omitempty"`
			KubernetesNamespace string `json:"namespace,omitempty"`
			KubernetesPod       string `json:"pod,omitempty"`
			KubernetesContainer string `json:"container,omitempty"`
		}
		allProcesses := []Process{}

		for _, i := range results.Items {
			processes := []Process{}
			json.Unmarshal([]byte(i.Status.Output), &processes)
			allProcesses = append(allProcesses, processes...)
		}
		if !processCollectorParamThreads {
			allProcessesTrimmed := []Process{}
			for _, i := range allProcesses {
				if i.Tgid == i.Pid {
					allProcessesTrimmed = append(allProcessesTrimmed, i)
				}
			}
			allProcesses = allProcessesTrimmed
		}

		sort.Slice(allProcesses, func(i, j int) bool {
			pi, pj := allProcesses[i], allProcesses[j]
			switch {
			case pi.KubernetesNamespace != pj.KubernetesNamespace:
				return pi.KubernetesNamespace < pj.KubernetesNamespace
			case pi.KubernetesPod != pj.KubernetesPod:
				return pi.KubernetesPod < pj.KubernetesPod
			case pi.KubernetesContainer != pj.KubernetesContainer:
				return pi.KubernetesContainer < pj.KubernetesContainer
			case pi.Comm != pj.Comm:
				return pi.Comm < pj.Comm
			case pi.Tgid != pj.Tgid:
				return pi.Tgid < pj.Tgid
			default:
				return pi.Pid < pj.Pid

			}
		})
		if params.OutputMode == utils.OutputModeJson {
			b, err := json.MarshalIndent(allProcesses, "", "  ")
			if err != nil {
				contextLogger.Fatalf("Error marshalling results: %s", err)
			}
			fmt.Printf("%s\n", b)
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
			if processCollectorParamThreads {
				fmt.Fprintln(w, "NAMESPACE\tPOD\tCONTAINER\tCOMM\tTGID\tPID\t")
				for _, p := range allProcesses {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t\n",
						p.KubernetesNamespace,
						p.KubernetesPod,
						p.KubernetesContainer,
						p.Comm,
						p.Tgid,
						p.Pid,
					)
				}
			} else {
				fmt.Fprintln(w, "NAMESPACE\tPOD\tCONTAINER\tCOMM\tPID\t")
				for _, p := range allProcesses {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t\n",
						p.KubernetesNamespace,
						p.KubernetesPod,
						p.KubernetesContainer,
						p.Comm,
						p.Pid,
					)
				}
			}
			w.Flush()
		}
	}

	utils.GenericTraceCommand("process-collector", &params, args, "Status", callback, nil)
}

func socketCollectorCmdRun(cmd *cobra.Command, args []string) {
	callback := func(contextLogger *log.Entry, nodes *corev1.NodeList, results *gadgetv1alpha1.TraceList) {
		allSockets := []socketcollectortypes.Event{}

		for _, i := range results.Items {
			var sockets []socketcollectortypes.Event
			json.Unmarshal([]byte(i.Status.Output), &sockets)
			allSockets = append(allSockets, sockets...)
		}

		sort.Slice(allSockets, func(i, j int) bool {
			si, sj := allSockets[i], allSockets[j]
			switch {
			case si.Event.Node != sj.Event.Node:
				return si.Event.Node < sj.Event.Node
			case si.Event.Namespace != sj.Event.Namespace:
				return si.Event.Namespace < sj.Event.Namespace
			case si.Event.Pod != sj.Event.Pod:
				return si.Event.Pod < sj.Event.Pod
			case si.Protocol != sj.Protocol:
				return si.Protocol < sj.Protocol
			case si.Status != sj.Status:
				return si.Status < sj.Status
			case si.LocalAddress != sj.LocalAddress:
				return si.LocalAddress < sj.LocalAddress
			case si.RemoteAddress != sj.RemoteAddress:
				return si.RemoteAddress < sj.RemoteAddress
			case si.LocalPort != sj.LocalPort:
				return si.LocalPort < sj.LocalPort
			default:
				return si.RemotePort < sj.RemotePort
			}
		})

		if params.OutputMode == utils.OutputModeJson {
			b, err := json.MarshalIndent(allSockets, "", "  ")
			if err != nil {
				contextLogger.Fatalf("Error marshalling results: %s", err)
			}
			fmt.Printf("%s\n", b)
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

			fmt.Fprintln(w, "NODE\tNAMESPACE\tPOD\tPROTOCOL\tLOCAL\tREMOTE\tSTATUS")

			for _, s := range allSockets {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s:%d\t%s:%d\t%s\n",
					s.Event.Node,
					s.Event.Namespace,
					s.Event.Pod,
					s.Protocol,
					s.LocalAddress,
					s.LocalPort,
					s.RemoteAddress,
					s.RemotePort,
					s.Status,
				)
			}
			w.Flush()
		}
	}

	utils.GenericTraceCommand("socket-collector", &params, args, "Status", callback, nil)
}
