package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

func main() {

	var (
		defaultTMP              string
		installPath             string
		mpmDownloadPath         string
		mpmURL                  string
		mpmDownloadNeeded       bool
		products                []string
		release                 string
		defaultInstallationPath string
		licenseFileUsed         bool
		licensePath             string
		mpmFullPath             string
	)
	mpmDownloadNeeded = true
	platform := runtime.GOOS
	redText := color.New(color.FgRed).SprintFunc()
	redBackground := color.New(color.BgRed).SprintFunc()

	// Reader to make using the command line not suck.
	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	// Setup for better Ctrl+C messaging. This is a channel to receive OS signals.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Start a goroutine to listen for signals.
	go func() {

		// Wait for the signal.
		<-signalChan

		// Handle the signal (in this case, simply exit the program.)
		fmt.Println(redBackground("\nExiting from user input..."))
		os.Exit(0)
	}()

	// Figure out your OS.
	switch platform {
	case "darwin":
		defaultTMP = "/tmp"
		switch runtime.GOARCH {
		case "amd64":
			mpmURL = "https://www.mathworks.com/mpm/maci64/mpm"
			platform = "macOSx64"
		case "arm64":
			mpmURL = "https://www.mathworks.com/mpm/maca64/mpm"
			platform = "macOSARM"
		}
	case "windows":
		defaultTMP = os.Getenv("TMP")
		mpmURL = "https://www.mathworks.com/mpm/win64/mpm"
	case "linux":
		defaultTMP = "/tmp"
		mpmURL = "https://www.mathworks.com/mpm/glnxa64/mpm"
	default:
		defaultTMP = "unknown"
		fmt.Println(redText("Your operating system is unrecognized. Exiting."))
		os.Exit(0)
	}

	// Figure out where you want actual MPM to go.
	for {
		fmt.Print("Enter the path to the directory where you would like MPM to download to. " +
			"Press Enter to use \"" + defaultTMP + "\"\n> ")
		mpmDownloadPath, err = rl.Readline()
		if err != nil {
			if err.Error() == "Interrupt" {
				fmt.Println(redText("Exiting from user input."))
			} else {
				fmt.Println(redText("Error reading line: ", err))
				continue
			}
			return
		}
		mpmDownloadPath = strings.TrimSpace(mpmDownloadPath)

		if mpmDownloadPath == "" {
			mpmDownloadPath = defaultTMP
		} else {
			_, err := os.Stat(mpmDownloadPath)
			if os.IsNotExist(err) {
				fmt.Printf("The directory \"%s\" does not exist. Do you want to create it? (y/n)\n> ", mpmDownloadPath)
				createDir, err := rl.Readline()
				if err != nil {
					if err.Error() == "Interrupt" {
						fmt.Println(redText("Exiting from user input."))
					} else {
						fmt.Println(redText("Error reading line: ", err))
						continue
					}
					return
				}
				createDir = strings.TrimSpace(createDir)

				// Don't ask me why I've only put this here so far.
				// I'll probably put it in other places that don't ask for file names/paths.
				if createDir == "exit" || createDir == "Exit" || createDir == "quit" || createDir == "Quit" {
					os.Exit(0)
				}

				if createDir == "y" || createDir == "Y" {
					err := os.MkdirAll(mpmDownloadPath, 0755)
					if err != nil {
						fmt.Println(redText("Failed to create the directory: ", err, "Please select a different directory."))
						continue
					}
					fmt.Println("Directory created successfully.")
				} else {
					fmt.Println("Directory creation skipped. Please select a different directory.")
					continue
				}
			} else if err != nil {
				fmt.Println(redText("Error checking the directory: ", err, "Please select a different directory."))
				continue
			}
		}

		// Check if MPM already exists in the selected directory.
		fileName := filepath.Join(mpmDownloadPath, "mpm")
		if platform == "windows" {
			fileName = filepath.Join(mpmDownloadPath, "mpm.exe")
		}
		_, err := os.Stat(fileName)
		for {
			if err == nil {
				fmt.Println("MPM already exists in this directory. Would you like to overwrite it?")
				overwriteMPM, err := rl.Readline()
				if err != nil {
					if err.Error() == "Interrupt" {
						fmt.Println(redText("Exiting from user input."))
					} else {
						fmt.Println(redText("Error reading line: ", err))
						continue
					}
					return
				}

				overwriteMPM = cleanInput(overwriteMPM)
				if overwriteMPM == "n" || overwriteMPM == "N" {
					fmt.Println("Skipping download.")
					mpmDownloadNeeded = false
					break
				}
				if overwriteMPM == "y" || overwriteMPM == "Y" {
					break
				} else {
					fmt.Println(redText("Invalid choice. Please enter either 'y' or 'n'."))
					continue
				}
			}
			break
		}

		// Download MPM.
		if mpmDownloadNeeded {
			fmt.Println("Beginning download of MPM. Please wait.")
			err = downloadFile(mpmURL, fileName)
			if err != nil {
				fmt.Println(redText("Failed to download MPM. ", err))
				continue
			}
			fmt.Println("MPM downloaded successfully.")
		}

		// Make sure you can actually execute MPM on Linux.
		if runtime.GOOS == "linux" {
			command := "chmod +x " + mpmDownloadPath + "/mpm"

			// Execute the command
			cmd := exec.Command("bash", "-c", command)
			err := cmd.Run()

			if err != nil {
				fmt.Println("Failed to execute the command: ", err)
				fmt.Print(". Either select a different directory, run this program with needed privileges, " +
					"or make modifications to MPM outside of this program.")
				continue
			}
		}
		break
	}

	// Ask the user which release they'd like to install.
	validReleases := []string{
		"R2017b", "R2018a", "R2018b", "R2019a", "R2019b", "R2020a", "R2020b",
		"R2021a", "R2021b", "R2022a", "R2022b", "R2023a", "R2023b", "R2024a",
	}
	defaultRelease := "R2024a"

	for {
		fmt.Printf("Enter which release you would like to install. Press Enter to select %s: ", defaultRelease)
		fmt.Print("\n> ")
		release, err = rl.Readline()
		if err != nil {
			if err.Error() == "Interrupt" {
				fmt.Println(redText("Exiting from user input."))
			} else {
				fmt.Println(redText("Error reading line: ", err))
				continue
			}
			return
		}

		release = strings.TrimSpace(release)
		if release == "" {
			release = defaultRelease
		}

		release = strings.ToLower(release)
		found := false
		for _, validRelease := range validReleases {
			if strings.ToLower(validRelease) == release {
				release = validRelease
				found = true
				break
			}
		}

		if found {
			break
		}

		fmt.Println(redText("Invalid release. Enter a release between R2017b-R2024a."))
	}

	for {
		// Product selection.
		fmt.Print("Enter the products you would like to install. Use the same syntax as MPM to specify products. " +
			"Press Enter to install all products.\n> ")
		productsInput, err := rl.Readline()
		if err != nil {
			if err.Error() == "Interrupt" {
				fmt.Println(redText("Exiting from user input."))
			} else {
				fmt.Println(redText("Error reading line: ", err))
				continue
			}
			return
		}

		productsInput = strings.TrimSpace(productsInput)

		var products []string
		var newProductsToAdd map[string]string
		var oldProductsToAdd map[string]string

		// Add some code below that will break up these 2 lists between the 3 Operating Systems because right now, this only reflects Linux. Yayyyyy.
		if productsInput == "" {

			// new products to add
			if platform == "windows" {
				newProductsToAdd = map[string]string{
					"R2024a": "", // No new products were added in R2024a.
					"R2023b": "Simulink_Fault_Analyzer Polyspace_Test",
					"R2023a": "MATLAB_Test C2000_Microcontroller_Blockset",
					"R2022b": "Medical_Imaging_Toolbox Simscape_Battery",
					"R2022a": "Wireless_Testbench Bluetooth_Toolbox DSP_HDL_Toolbox Requirements_Toolbox Industrial_Communication_Toolbox",
					"R2021b": "Signal_Integrity_Toolbox RF_PCB_Toolbox",
					"R2021a": "Satellite_Communications_Toolbox DDS_Blockset",
					"R2020b": "UAV_Toolbox Radar_Toolbox Lidar_Toolbox Deep_Learning_HDL_Toolbox",
					"R2020a": "Simulink_Compiler Motor_Control_Blockset MATLAB_Web_App_Server Wireless_HDL_Toolbox",
					"R2019b": "ROS_Toolbox Simulink_PLC_Coder Navigation_Toolbox",
					"R2019a": "System_Composer SoC_Blockset SerDes_Toolbox Reinforcement_Learning_Toolbox Audio_Toolbox Mixed-Signal_Blockset AUTOSAR_Blockset MATLAB_Parallel_Server Polyspace_Bug_Finder_Server Polyspace_Code_Prover_Server Automated_Driving_Toolbox Computer_Vision_Toolbox",
					"R2018b": "Communications_Toolbox Simscape_Electrical Sensor_Fusion_and_Tracking_Toolbox Deep_Learning_Toolbox 5G_Toolbox WLAN_Toolbox LTE_Toolbox",
					"R2018a": "Predictive_Maintenance_Toolbox Vehicle_Dynamics_Blockset",
					"R2017b": "Aerospace_Blockset Aerospace_Toolbox Antenna_Toolbox Bioinformatics_Toolbox Control_System_Toolbox Curve_Fitting_Toolbox DSP_System_Toolbox Data_Acquisition_Toolbox Database_Toolbox Datafeed_Toolbox Econometrics_Toolbox Embedded_Coder Filter_Design_HDL_Coder Financial_Instruments_Toolbox Financial_Toolbox Fixed-Point_Designer Fuzzy_Logic_Toolbox GPU_Coder Global_Optimization_Toolbox HDL_Coder HDL_Verifier Image_Acquisition_Toolbox Image_Processing_Toolbox Instrument_Control_Toolbox MATLAB MATLAB_Coder MATLAB_Compiler MATLAB_Compiler_SDK MATLAB_Production_Server MATLAB_Report_Generator Mapping_Toolbox Model_Predictive_Control_Toolbox Model-Based_Calibration_Toolbox OPC_Toolbox Optimization_Toolbox Parallel_Computing_Toolbox Partial_Differential_Equation_Toolbox Phased_Array_System_Toolbox Polyspace_Bug_Finder Polyspace_Code_Prover Powertrain_Blockset RF_Blockset RF_Toolbox Risk_Management_Toolbox Robotics_System_Toolbox Robust_Control_Toolbox Signal_Processing_Toolbox SimBiology SimEvents Simscape Simscape_Driveline Simscape_Fluids Simscape_Multibody Simulink Simulink_3D_Animation Simulink_Check Simulink_Coder Simulink_Control_Design Simulink_Coverage Simulink_Design_Optimization Simulink_Design_Verifier Simulink_Desktop_Real-Time Simulink_PLC_Coder Simulink_Real-Time Simulink_Report_Generator Simulink_Test Spreadsheet_Link Stateflow Statistics_and_Machine_Learning_Toolbox Symbolic_Math_Toolbox System_Identification_Toolbox Text_Analytics_Toolbox Vehicle_Network_Toolbox Vision_HDL_Toolbox Wavelet_Toolbox",
				}

			} else if platform == "linux" {
				newProductsToAdd = map[string]string{
					"R2024a": "", // No new products were added in R2024a.
					"R2023b": "Simulink_Fault_Analyzer Polyspace_Test Simulink_Desktop_Real-Time",
					"R2023a": "MATLAB_Test C2000_Microcontroller_Blockset",
					"R2022b": "Medical_Imaging_Toolbox Simscape_Battery",
					"R2022a": "Wireless_Testbench Simulink_Real-Time Bluetooth_Toolbox DSP_HDL_Toolbox Requirements_Toolbox Industrial_Communication_Toolbox",
					"R2021b": "Signal_Integrity_Toolbox RF_PCB_Toolbox",
					"R2021a": "Satellite_Communications_Toolbox DDS_Blockset",
					"R2020b": "UAV_Toolbox Radar_Toolbox Lidar_Toolbox Deep_Learning_HDL_Toolbox",
					"R2020a": "Simulink_Compiler Motor_Control_Blockset MATLAB_Web_App_Server Wireless_HDL_Toolbox",
					"R2019b": "ROS_Toolbox Simulink_PLC_Coder Navigation_Toolbox",
					"R2019a": "System_Composer SoC_Blockset SerDes_Toolbox Reinforcement_Learning_Toolbox Audio_Toolbox Mixed-Signal_Blockset AUTOSAR_Blockset MATLAB_Parallel_Server Polyspace_Bug_Finder_Server Polyspace_Code_Prover_Server Automated_Driving_Toolbox Computer_Vision_Toolbox",
					"R2018b": "Communications_Toolbox Simscape_Electrical Sensor_Fusion_and_Tracking_Toolbox Deep_Learning_Toolbox 5G_Toolbox WLAN_Toolbox LTE_Toolbox",
					"R2018a": "Predictive_Maintenance_Toolbox Vehicle_Network_Toolbox Vehicle_Dynamics_Blockset",
					"R2017b": "Aerospace_Blockset Aerospace_Toolbox Antenna_Toolbox Bioinformatics_Toolbox Control_System_Toolbox Curve_Fitting_Toolbox DSP_System_Toolbox Database_Toolbox Datafeed_Toolbox Econometrics_Toolbox Embedded_Coder Filter_Design_HDL_Coder Financial_Instruments_Toolbox Financial_Toolbox Fixed-Point_Designer Fuzzy_Logic_Toolbox GPU_Coder Global_Optimization_Toolbox HDL_Coder HDL_Verifier Image_Acquisition_Toolbox Image_Processing_Toolbox Instrument_Control_Toolbox MATLAB MATLAB_Coder MATLAB_Compiler MATLAB_Compiler_SDK MATLAB_Production_Server MATLAB_Report_Generator Mapping_Toolbox Model_Predictive_Control_Toolbox Optimization_Toolbox Parallel_Computing_Toolbox Partial_Differential_Equation_Toolbox Phased_Array_System_Toolbox Polyspace_Bug_Finder Polyspace_Code_Prover Powertrain_Blockset RF_Blockset RF_Toolbox Risk_Management_Toolbox Robotics_System_Toolbox Robust_Control_Toolbox Signal_Processing_Toolbox SimBiology SimEvents Simscape Simscape_Driveline Simscape_Fluids Simscape_Multibody Simulink Simulink_3D_Animation Simulink_Check Simulink_Coder Simulink_Control_Design Simulink_Coverage Simulink_Design_Optimization Simulink_Design_Verifier Simulink_Report_Generator Simulink_Test Stateflow Statistics_and_Machine_Learning_Toolbox Symbolic_Math_Toolbox System_Identification_Toolbox Text_Analytics_Toolbox Vision_HDL_Toolbox Wavelet_Toolbox",
				}

			} else if platform == "macOSx64" {
				newProductsToAdd = map[string]string{
					"R2024a": "", // No new products were added in R2024a.
					"R2023b": "Simulink_Fault_Analyzer Polyspace_Test",
					"R2023a": "MATLAB_Test",
					"R2022b": "Medical_Imaging_Toolbox Simscape_Battery",
					"R2022a": "Bluetooth_Toolbox DSP_HDL_Toolbox Requirements_Toolbox Industrial_Communication_Toolbox",
					"R2021b": "RF_PCB_Toolbox",
					"R2021a": "Satellite_Communications_Toolbox DDS_Blockset",
					"R2020b": "UAV_Toolbox Radar_Toolbox Lidar_Toolbox",
					"R2020a": "Simulink_Compiler Motor_Control_Blockset MATLAB_Web_App_Server Wireless_HDL_Toolbox",
					"R2019b": "ROS_Toolbox Simulink_PLC_Coder Navigation_Toolbox",
					"R2019a": "System_Composer SoC_Blockset SerDes_Toolbox Reinforcement_Learning_Toolbox Audio_Toolbox Mixed-Signal_Blockset AUTOSAR_Blockset Polyspace_Bug_Finder_Server Polyspace_Code_Prover_Server Automated_Driving_Toolbox Computer_Vision_Toolbox",
					"R2018b": "Communications_Toolbox Simscape_Electrical Sensor_Fusion_and_Tracking_Toolbox Deep_Learning_Toolbox 5G_Toolbox WLAN_Toolbox LTE_Toolbox",
					"R2018a": "Predictive_Maintenance_Toolbox Vehicle_Dynamics_Blockset",
					"R2017b": "Aerospace_Blockset Aerospace_Toolbox Antenna_Toolbox Audio_System_Toolbox Automated_Driving_System_Toolbox Bioinformatics_Toolbox Computer_Vision_System_Toolbox Control_System_Toolbox Curve_Fitting_Toolbox DSP_System_Toolbox Database_Toolbox Datafeed_Toolbox Econometrics_Toolbox Embedded_Coder Filter_Design_HDL_Coder Financial_Instruments_Toolbox Financial_Toolbox Fixed-Point_Designer Fuzzy_Logic_Toolbox Global_Optimization_Toolbox HDL_Coder Image_Acquisition_Toolbox Image_Processing_Toolbox Instrument_Control_Toolbox MATLAB MATLAB_Coder MATLAB_Compiler MATLAB_Compiler_SDK MATLAB_Production_Server MATLAB_Report_Generator Mapping_Toolbox Model_Predictive_Control_Toolbox Optimization_Toolbox Parallel_Computing_Toolbox Partial_Differential_Equation_Toolbox Phased_Array_System_Toolbox Polyspace_Bug_Finder Polyspace_Code_Prover Powertrain_Blockset RF_Blockset RF_Toolbox Risk_Management_Toolbox Robotics_System_Toolbox Robust_Control_Toolbox Signal_Processing_Toolbox SimBiology SimEvents Simscape Simscape_Driveline Simscape_Fluids Simscape_Multibody Simulink Simulink_3D_Animation Simulink_Check Simulink_Coder Simulink_Control_Design Simulink_Coverage Simulink_Design_Optimization Simulink_Design_Verifier Simulink_Desktop_Real-Time Simulink_Report_Generator Simulink_Requirements Simulink_Test Stateflow Statistics_and_Machine_Learning_Toolbox Symbolic_Math_Toolbox System_Identification_Toolbox Text_Analytics_Toolbox Trading_Toolbox Wavelet_Toolbox",
				}

			} else if platform == "macOSARM" {
				newProductsToAdd = map[string]string{
					"R2024a": "", // No new products were added in R2024a.
					"R2023b": "5G_Toolbox AUTOSAR_Blockset Aerospace_Blockset Aerospace_Toolbox Antenna_Toolbox Audio_Toolbox Automated_Driving_Toolbox Bioinformatics_Toolbox Bluetooth_Toolbox Communications_Toolbox Computer_Vision_Toolbox Control_System_Toolbox Curve_Fitting_Toolbox DDS_Blockset DSP_HDL_Toolbox DSP_System_Toolbox Database_Toolbox Datafeed_Toolbox Deep_Learning_Toolbox Econometrics_Toolbox Embedded_Coder Filter_Design_HDL_Coder Financial_Instruments_Toolbox Financial_Toolbox Fixed-Point_Designer Fuzzy_Logic_Toolbox Global_Optimization_Toolbox HDL_Coder Image_Acquisition_Toolbox Image_Processing_Toolbox Industrial_Communication_Toolbox Instrument_Control_Toolbox LTE_Toolbox Lidar_Toolbox MATLAB MATLAB_Coder MATLAB_Compiler MATLAB_Compiler_SDK MATLAB_Report_Generator MATLAB_Test Mapping_Toolbox Medical_Imaging_Toolbox Mixed-Signal_Blockset Model_Predictive_Control_Toolbox Motor_Control_Blockset Navigation_Toolbox Optimization_Toolbox Parallel_Computing_Toolbox Partial_Differential_Equation_Toolbox Phased_Array_System_Toolbox Powertrain_Blockset Predictive_Maintenance_Toolbox RF_Blockset RF_PCB_Toolbox RF_Toolbox ROS_Toolbox Radar_Toolbox Reinforcement_Learning_Toolbox Requirements_Toolbox Risk_Management_Toolbox Robotics_System_Toolbox Robust_Control_Toolbox Satellite_Communications_Toolbox Sensor_Fusion_and_Tracking_Toolbox SerDes_Toolbox Signal_Processing_Toolbox SimBiology SimEvents Simscape Simscape_Battery Simscape_Driveline Simscape_Electrical Simscape_Fluids Simscape_Multibody Simulink Simulink_3D_Animation Simulink_Check Simulink_Coder Simulink_Compiler Simulink_Control_Design Simulink_Coverage Simulink_Design_Optimization Simulink_Design_Verifier Simulink_Fault_Analyzer Simulink_PLC_Coder Simulink_Report_Generator Simulink_Test Stateflow Statistics_and_Machine_Learning_Toolbox Symbolic_Math_Toolbox System_Composer System_Identification_Toolbox Text_Analytics_Toolbox UAV_Toolbox Vehicle_Dynamics_Blockset WLAN_Toolbox Wavelet_Toolbox Wireless_HDL_Toolbox",
				}
			}

			// The actual for loop that goes through the list above.
			for releaseLoop, product := range newProductsToAdd {
				if release >= releaseLoop {
					products = append(products, strings.Fields(product)...)
				}
			}

			// old products to add
			if platform == "windows" {
				oldProductsToAdd = map[string]string{
					"R2021b": "Simulink_Requirements",
					"R2020b": "Trading_Toolbox",
					"R2019b": "LTE_HDL_Toolbox",
					"R2018b": "Audio_System_Toolbox Automated_Driving_System_Toolbox Computer_Vision_System_Toolbox MATLAB_Distributed_Computing_Server",
					"R2018a": "Communications_System_Toolbox LTE_System_Toolbox Neural_Network_Toolbox Simscape_Electronics Simscape_Power_Systems WLAN_System_Toolbox",
				}

			} else if platform == "linux" {
				oldProductsToAdd = map[string]string{
					"R2021b": "Simulink_Requirements",
					"R2020b": "Trading_Toolbox",
					"R2019b": "LTE_HDL_Toolbox",
					"R2018b": "Audio_System_Toolbox Automated_Driving_System_Toolbox Computer_Vision_System_Toolbox MATLAB_Distributed_Computing_Server",
					"R2018a": "Communications_System_Toolbox LTE_System_Toolbox Neural_Network_Toolbox Simscape_Electronics Simscape_Power_Systems WLAN_System_Toolbox",
				}

			} else if platform == "macOSx64" {
				oldProductsToAdd = map[string]string{
					"R2021b": "Simulink_Requirements MATLAB_Parallel_Server",
					"R2020b": "Trading_Toolbox",
					"R2019b": "LTE_HDL_Toolbox",
					"R2018b": "Audio_System_Toolbox Automated_Driving_System_Toolbox Computer_Vision_System_Toolbox MATLAB_Distributed_Computing_Server",
					"R2018a": "Communications_System_Toolbox LTE_System_Toolbox Neural_Network_Toolbox Simscape_Electronics Simscape_Power_Systems WLAN_System_Toolbox",
				}

			} else if platform == "macOSARM" {
				// No oldProductsToAdd for macOSARM
			}

			// The actual for loop that goes through the list above. Note that it uses the same logic, just <= instead of >=.
			for releaseLoop, product := range oldProductsToAdd {
				if release <= releaseLoop {
					products = append(products, strings.Fields(product)...)
				}
			}
		} else if productsInput == "parallel_products" {

			products = []string{"MATLAB", "Parallel_Computing_Toolbox", "MATLAB_Parallel_Server"}

		} else {
			products = strings.Fields(productsInput)
		}
		break
	}

	// Set the default installation path based on your OS.
	if platform == "macOSx64" || platform == "macOSARM" {
		defaultInstallationPath = "/Applications/MATLAB_" + release
	}
	if platform == "windows" {
		defaultInstallationPath = "C:\\Program Files\\MATLAB\\" + release
	}
	if platform == "linux" {
		defaultInstallationPath = "/usr/local/MATLAB/" + release
	}

	for {
		fmt.Print("Enter the full path where you would like to install these products. "+
			"Press Enter to install to default path: \"", defaultInstallationPath, "\"\n> ")

		var installPathInput string // For whatever reason, if I try to use installPath with ReadLine, it erases its value, so I'm using installPathInput instead.
		installPathInput, err := rl.Readline()
		if err != nil {
			if err.Error() == "Interrupt" {
				fmt.Println(redText("Exiting from user input."))
			} else {
				fmt.Println(redText("Error reading line: ", err))
				continue
			}
			return
		}

		installPath = installPathInput
		installPath = strings.TrimSpace(installPath)

		if installPath == "" {
			installPath = defaultInstallationPath
		}
		break
	}

	// Add some code to check the following:
	// - If you have permissions to read/write there

	// Optional license file selection.
	for {
		fmt.Print("If you have a license file you'd like to include in your installation, " +
			"please provide the full path to the existing license file.\n> ")

		licensePath, err = rl.Readline()
		if err != nil {
			if err.Error() == "Interrupt" {
				fmt.Println(redText("Exiting from user input."))
			} else {
				fmt.Println(redText("Error reading line: ", err))
				continue
			}
			return
		}
		licensePath = strings.TrimSpace(licensePath)

		if licensePath == "" {
			licenseFileUsed = false
			break
		} else {

			// Check if the license file exists and has the correct extension.
			_, err := os.Stat(licensePath)
			if err != nil {
				fmt.Println(redText("Error: ", err))
				continue
			} else if !strings.HasSuffix(licensePath, ".dat") && !strings.HasSuffix(licensePath, ".lic") {
				fmt.Println(redText("Invalid file extension. Please provide a file with .dat or .lic extension."))
				continue
			} else {
				licenseFileUsed = true
				break
			}
		}
	}

	if runtime.GOOS == "darwin" {
		mpmFullPath = mpmDownloadPath + "/mpm"
	}
	if runtime.GOOS == "windows" {
		mpmFullPath = mpmDownloadPath + "\\mpm.exe"
	}
	if runtime.GOOS == "linux" {
		mpmFullPath = mpmDownloadPath + "/mpm"
	}

	// Construct the command and arguments to launch MPM.
	cmdArgs := []string{
		mpmFullPath,
		"install",
		"--release=" + release,
		"--destination=" + installPath,
		"--products",
	}
	cmdArgs = append(cmdArgs, products...)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(redText("Error executing MPM. See the error above for more information. ", err))
	}

	// Create the licenses directory and the file specified, if you specified one.
	if licenseFileUsed {

		// Create the directory.
		licensesInstallationDirectory := filepath.Join(installPath, "licenses")
		err := os.Mkdir(licensesInstallationDirectory, 0755)
		if err != nil {
			fmt.Println(redText("Error creating \"licenses\" directory: ", err))
		}

		// Copy the license file to the "licenses" directory.
		licenseFile := filepath.Base(licensePath)
		destPath := filepath.Join(licensesInstallationDirectory, licenseFile)

		src, err := os.Open(licensePath)
		if err != nil {
			fmt.Println(redText("Error opening license file: ", err))
		}
		defer src.Close()

		dest, err := os.Create(destPath)
		if err != nil {
			fmt.Println(redText("Error creating destination file: ", err))
		}
		defer dest.Close()

		_, err = io.Copy(dest, src)
		if err != nil {
			fmt.Println(redText("Error copying license file: ", err))
		}
	}
	// Next steps:
	// - May need to chmod mpm on Linux. Should test this soon.
	// - Ask which products they'd like to install.
	// - Painstakingly find out all products can be installed for each release on Windows and macOS.
	// - Figure out the most efficient way to do the above, including Linux.
	// - Ask for an installation path.
	// - Ask if you want to use a license file.
	// - Kick off installation.
	// - Place the license file if you asked to use one.
}

// Clean input function.
func cleanInput(input string) string {
	return strings.TrimSpace(input)
}

// Function to download a file from the given URL and save it to the specified path.
func downloadFile(url string, filePath string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}

// Make sure the products you've specified exist.
func checkProductsExist(inputProducts []string, availableProducts string) bool {
	availableProductsList := strings.Fields(availableProducts)
	productSet := make(map[string]struct{}, len(availableProductsList))
	for _, product := range availableProductsList {
		productSet[product] = struct{}{}
	}

	for _, inputProduct := range inputProducts {
		if _, exists := productSet[inputProduct]; !exists {
			return false
		}
	}
	return true
}
