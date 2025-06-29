# Verapack

Verapack is a utility that automates and simplifies running Veracode **SAST** & **SCA Upload** scans from your local machine.

I built this tool to address a use case at a client where certain teams lacked a CI/CD pipeline for their applications, forcing them to run SAST scans manually. As a result, most of these teams had to run the scans for each of their multiple application once or more per month to remain within policy. This tedious and time-consuming process was prone to error, and if they missed an application, it could break their policy compliance streak. I found that this made Veracode and application security, in general, a negative experience for those teams. That's what I've tried to address with this tool.

The scope of this tool is not to replace automated security testing. This tool is intended to make developers' lives easier until either automation/a pipeline is introduced or the application is decommissioned (depending on the lifecycle stage of the application).

## ✨ Key Features

- Convenient setup process that simplifies setting up local credential files and installs Veracode tools.
- Manage all scanning configurations from a central `config.yaml` file.
- Run a **Policy**-, **Sandbox**-, **Sandbox Promotion** or mixed scans asynchronously.
- You can leave the scans running in the background and view the results in the application once the scans complete.
- You can use a URL to the repo, path to the local project or provide a list of pre-built artefacts to scan for each of the different applications.
- Use the [Veracode auto-packager](https://docs.veracode.com/r/About_auto_packaging) to automatically package the applications.
- Check the latest versions of the installed tooling and automatically update them.
- Automatically refresh API credentials and update local credential files. (API credentials expire after 1 year)

## 🧱 Prerequisites

- **Windows amd64**, the Verapack tool is currently only designed to work on this OS/Architecture.
- **Java 8, 11 or 17**, this is required for one of the Veracode tools.
- **git**, git must be installed and added to the user's path. Verapack currently only supports git-based repositories.
- Please review the [language support](https://docs.veracode.com/r/About_auto_packaging#supported-languages) for the **auto-packager**.

> [!NOTE]
> The tool does not require administrative rights to use.

## 📥 Installation

To install Verapack, you can download the latest build from the [releases](https://github.com/DanCreative/verapack/releases) tab. Then once the archive has been downloaded and extracted, you can move the binary inside to a directory that makes sense to you and add said directory path to your user account `PATH` environment variable. E.g. `%Appdata%\verapack`

## 📖 Basic User Guide

### 1. Setup: Part I

Once Verapack has been installed, run the first-time setup command:

```powershell
.\verapack setup
```

<img width="600" alt="A GIF demonstrating the setup process" src=".vhs/output/setup.gif">

The setup process will do the following:

- It will check that all prerequisites are met on your machine.
- It will prompt you for your API credentials if a credentials file is not already present.
- It will add the provided credentials to new credential files. (both the old .ini- and the new .yaml formats. It does this so that the developer can easily use the underlying tools without Verapack)
- It will create a template of the config file.
- It will install the [Veracode Java API wrapper](https://docs.veracode.com/r/t_working_with_java_wrapper), the [Veracode CLI](https://docs.veracode.com/r/Veracode_CLI) and the [SCA CLI Agent](https://docs.veracode.com/r/Using_the_Veracode_SCA_Command_Line_Agent).

### 2. Setup: Part II

Once the setup has completed successfully, it will show you where the config file is located. You can then open that file using your favourite editor and add the applications that you wish to scan.

The config file is in YAML format, and is structured like below. Please see an example template file [here](https://github.com/DanCreative/verapack/tree/main/internal/verapack/config.yaml).

#### Config file YAML Schema

<details>

<summary>Click here to expand</summary>

<br>

Field Name | Field Type | Required | Description
--- | --- | --- | ---
default | $${\color{lightgreen}Application}$$ | false | The default section will contain all of the default values for the settings that will be applied to all application specified in the applications section.
applications | $${Array \space of \color{lightgreen}Application}$$ | true | The applications section will contain a list of your application profiles. Settings set here will override the default values set in the default section.

<br>

$${\color{lightgreen}Application}$$

Field Name | Field Type | Required | Description
--- | --- | --- | ---
app_name | $${\color{lightblue}string}$$ | true | The name of the application profile on the Veracode platform.
package_source | $${\color{lightblue}string}$$ | either ```this``` field or ```artefact_paths``` is required | Location of the source to package based on the target type. If the type is directory, enter the path to a local directory. If the type is repo, enter the URL to a Git version control system. For each application, you can either set ```this``` field or the ```artefact_paths``` field (but not both or neither). Setting this field will use the auto-packager. The value must be a valid directory or URL.
artefact_paths | $${Array \space of \color{lightblue}string}$$ | either ```this``` field or ```package_source``` is required | A list of paths to specific files or directories that you want to upload for scanning. For each application, you can either set ```this``` field or the ```package_source``` field (but not both or neither). Setting this field will bypass auto-packager and upload the files/directories and sub-directories directly. The values must be valid directory- or file paths.
branch | $${\color{lightblue}string}$$ | false | Name of the specific branch that you want to scan.
verbose | $${\color{pink}bool}$$ | false | Increase output verbosity.
auto_cleanup | $${\color{pink}bool}$$ | false | Automatically remove any packaged artefacts after scanning completes.
type | $${\color{lightblue}string}$$ | false | Specifies the target type you want to package. This is used with ```package_source``` to automatically package either a repo or a local directory. The values can be: ```directory``` or ```repo```. The default value is ```directory```.
strict | $${\color{pink}bool}$$ | false | If this field is true, the packaging step will fail on application build failure.
create_profile | $${\color{pink}bool}$$ | false | Create a new application profile if one with the name set in ```app_name``` does not exist already.
sandbox_name | $${\color{lightblue}string}$$ | false | Name of the sandbox to use when running a sandbox scan or promoting a sandbox scan. If a sandbox with this name does not exist, it will be created.
version | $${\color{lightblue}string}$$ | false | Name or version of the build that you want to scan. This will be used as the scan name. If omitted, the current date-time in this format: "02 Jan 2006 15:04PM Static" will be used.
wait_for_result | $${\color{pink}bool}$$ | false | Wait for the scan to complete and return the status of the scan. ```scan_timeout``` and ```scan_polling_interval``` can optionally be set to customize the behaviour.
scan_timeout | $${\color{orange}int}$$ | false | Number of minutes to wait for the scan to complete. Only applicable when ```wait_for_result``` is set. The default value is: 120
scan_polling_interval | $${\color{orange}int}$$ | false | Interval, in seconds, to poll for the status of a running scan. Only applicable when ```wait_for_result``` is set. The value can be between: 30 - 120. The default value is: 30

</details>

### 3. Scanning

Run below command to start policy scans for the applications specified in the config file:

```powershell
.\verapack scan policy
```

Run below command to start sandbox scans:

```powershell
.\verapack scan sandbox
```

> [!NOTE]  
> If some of the applications do not have the sandbox_name field set, the user will be given a choice to proceed with the valid applications, or to cancel before starting.

> [!IMPORTANT]
> The user will require permission to create new sandboxes if they do not already exist.

Run below command to promote the latest sandbox scans in all of the sandboxes defined in the config file, to policy scans:

> [!NOTE]  
> If some of the applications do not have the sandbox_name field set, the user will be given a choice to proceed with the valid applications, to cancel before starting or to run a policy scan for the invalid applications.

> [!IMPORTANT]
> The sandboxes must already contain at least one scan otherwise promotion for that application and sandbox will fail.

```powershell
.\verapack scan promote
```

When running any of these commands, the user will be shown a report card where they can track the progress of the different steps and review the corresponding logs. Please see a demonstration below:

<img width="600" alt="A GIF demonstrating the report card when scanning" src=".vhs/output/scan-policy.gif">

While the user is on the report card screen, they can press below buttons to navigate through the results:

Keys | Action
---|---
<kbd>s</kbd> | Show the log output for the task that the user has selected.
<kbd>left</kbd>, <kbd>right</kbd>, <kbd>up</kbd>, <kbd>down</kbd> | The user can use the arrow keys to move around.
<kbd>t</kbd>, <kbd>g</kbd> | Page up or down on the list of applications.
<kbd>b</kbd>, <kbd>f</kbd> | Page up or down in the log output, if it is open.
<kbd>k</kbd>, <kbd>j</kbd> | Go up or down one line in the log output, if it is open.
<kbd>ctrl + c</kbd> | Quit the program.
<kbd>?</kbd> | See a full list of the keys available.

### 4. Stay up to date

You can run below command to check what versions of the tools are currently installed and to check if they are up to date.

```powershell
.\verapack -v
```

<img width="600" alt="A GIF demonstrating the version printer" src=".vhs/output/version-print.gif">

If there is a new version of the tools available, you can run below command to update those tools to the latest version.

```powershell
.\verapack update
```

<img width="600" alt="A GIF demonstrating the update command" src=".vhs/output/update.gif">

### 5. Credential Management

Veracode API credentials expire after one year. You can run below command to automatically refresh your credentials and to add the new ones to your local credential files.

```powershell
.\verapack credentials refresh
```

<img width="600" alt="A GIF demonstrating the credentials refresh command" src=".vhs/output/credentials-refresh.gif">

Alternatively, you can also manually set your local credentials using below command:

```powershell
.\verapack credentials configure
```

<img width="600" alt="A GIF demonstrating the credentials refresh command" src=".vhs/output/credentials-configure.gif">

## 🥞 Technologies & 📜 Licenses

Verapack and accompanied documentation is provided with the [MIT](https://github.com/DanCreative/verapack/tree/main/LICENSE) license.

Its dependencies **currently** make use of below licenses. Please review the licenses using the links, as they may change in the future which could render below information outdated.

Application dependencies:

Provider | Package | Usage | License
--- | --- | --- | ---
First-Party | [DanCreative/veracode-go](github.com/DanCreative/veracode-go) | API SDK | MIT
Veracode | [Veracode CLI](https://docs.veracode.com/r/Veracode_CLI) | Auto-packaging applications | Proprietary, Free to use
| | [vosp-api-wrappers-java](https://central.sonatype.com/artifact/com.veracode.vosp.api.wrappers/vosp-api-wrappers-java) | Uploading application builds | MIT
Charm | [charmbracelet/bubbletea](github.com/charmbracelet/bubbletea) | TUI framework | MIT
| | [charmbracelet/lipgloss](github.com/charmbracelet/lipgloss) | TUI styling | MIT
| | [charmbracelet/bubbles](github.com/charmbracelet/bubbles) | TUI component library | MIT
| | [charmbracelet/x/term](github.com/charmbracelet/x/term) | Terminal utilities & helpers | MIT
Other | [go-playground/validator](github.com/go-playground/validator/v10) | Config file validation | MIT
|| [goccy/go-yaml](github.com/goccy/go-yaml) | YAML parsing | MIT
|| [urfave/cli](github.com/urfave/cli/v2) | CLI framework | MIT

Other cool tools that I want to mention here:

- I used [charmbracelet/vhs](https://github.com/charmbracelet/vhs) to record the .GIFs of the application using its scripting language.
- I am using [softprops/action-gh-release](https://github.com/softprops/action-gh-release) to automatically create releases from the Github workflow when I push a new tag.
