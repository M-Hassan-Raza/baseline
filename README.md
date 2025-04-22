```markdown
# BASELINE v0.1: Your Terminal Baseline Test

## Amber Glow for a Grey Existence

In the flickering neon glow and perpetual damp of the terminal landscape, you require tools. Not flashy, over-engineered nonsense, but something reliable. Something that cuts through the digital smog. Something... baseline.

This is it. A simple dashboard, built from necessity, designed to present critical system parameters and arbitrary personal tasks in a pleasing, if somewhat melancholic, amber-on-black aesthetic reminiscent of... well, never mind what. It exists. That is sufficient.

## Why?

Look, the terminal doesn't have to be entirely devoid of... character. Or basic utility beyond executing commands that mostly fail anyway. Staring into a blank screen awaiting instructions is inefficient. Knowing your system isn't actively melting, whether it's raining outside (it probably is), and the pressing futility of your task list – that's valuable context. This provides that context. With amber.

It's a small act of control in a world determined to strip you of it. Plus, it looks pretty cool in a grim sort of way.

## Features (Necessary Components for Survival)

*   **System Status:** Track core vitals – CPU, memory, disk usage, network traffic. Ensure your machine isn't about to declare independence.
*   **Top Processes:** Identify the processes most aggressively consuming resources. Terminate with prejudice if required (elsewhere).
*   **Weather Report:** Get the atmospheric conditions for a specified location. Crucial for deciding if an unnecessary trip outside is even remotely viable. (Requires configuration. The system cannot guess the weather.)
*   **Current Time & Calendar:** A stark reminder of the relentless passage of temporal units. Includes a basic calendar and upcoming... events.
*   **Task List (TODOs):** Document the minor, often meaningless, tasks assigned to your unit. Add, toggle, delete, and prioritize them. A simulation of purpose.
*   **Notifications:** fleeting messages from the system, usually detailing minor errors or questionable successes.
*   **Command Interface:** Direct control. Issue instructions, change settings, or merely attempt to solicit help.
*   **Customizable Theme:** If the amber becomes too existentially heavy, other basic hues are available (though frankly, why would you?).

## Requirements (Do Not Fail)

To initiate this sequence, ensure these core components are integrated:

```bash
pip install rich requests python-dateutil psutil python-dotenv
# On Windows, you might also need:
# pip install msvcrt
# If you want functional keyboard input on non-Windows, you might need curses.
# (This is outside the standard requirements list, see the code for specifics)
# pip install keyboard # Code mentions 'keyboard' but isn't directly used
```

*   `rich`: For the visual structure and non-standard text rendering. It makes the amber glow possible.
*   `requests`: To retrieve external data (like the weather). The system needs to talk to the outside, reluctantly.
*   `python-dateutil`: To parse dates. Time is complex.
*   `psutil`: To interface with the underlying system's organic and inorganic components.
*   `python-dotenv`: To load parameters from the `.env` file. Critical for calibration.

*(Note: The code mentions `msvcrt` and `curses` for keyboard input. The requirements list provided was missing `curses`. Functional keyboard input beyond basic commands might require manual installation or system setup depending on your OS. Consider this... a feature.)*

## Installation (Initiating Protocol 7B)

Acquire the source code. Place it somewhere... discreet.

```bash
git clone https://github.com/your_repo_name/baseline.git # (Or however you got this)
cd baseline
```

## Configuration (Calibrating Your Reality)

Create a file named `.env` in the same directory as the script. This file contains... parameters. Adjust them as required.

```dotenv
WEATHER_API_KEY=YOUR_API_KEY_HERE
WEATHER_LOCATION=Your City Name
THEME=amber # Optional: 'green', 'blue', etc.
```

*   `WEATHER_API_KEY`: Obtain this from a data provider (e.g., WeatherAPI.com). If left as `YOUR_API_KEY_HERE`, sample data will be displayed. The system operates on assumptions when data is unavailable.
*   `WEATHER_LOCATION`: Specify the coordinates or name of the region for atmospheric monitoring.
*   `THEME`: Modify the primary visual frequency. `amber` is default and recommended for optimal... mood.

## Operation Manual (Usage)

Execute the primary script file:

```bash
python baseline.py
```

The interface should appear, filling your terminal with controlled information streams.

**Direct Control Interfaces (Keyboard Shortcuts):**

When not in command input mode (i.e., not typing after '>'):

*   `n`: Initiate New Task Input. Add another burden to the list.
*   `t`: Toggle Status. Mark the first incomplete task as done. A fleeting victory.
*   `d`: Delete Task. Purge the first completed task from history. Erasure.
*   `p`: Prioritize Task. Cycle priority of the first incomplete task. Rearranging deck chairs.
*   `q`: Quit. Terminate process. Escape.
*   `: `: Enter Command Mode. Direct interface access.
*   `?`: Help. Display available keyboard commands (a futile gesture).
*   `Tab`: Switch Panel Focus (Not fully implemented in provided code, consider it a future directive).

**Command Mode (`:`)**

Enter command mode by typing `:`. The cursor appears in the footer. Type commands followed by Enter:

*   `help` or `?`: List available commands.
*   `exit` or `quit`: Terminate the program.
*   `clear`: Erase notification history.
*   `shortcut`: Display keyboard shortcuts.
*   `theme [name]`: Attempt to change the color scheme (`amber`, `green`, `blue`).
*   `todo add [text]`: Add a task via the command line.
*   `todo toggle [index]`: Toggle the status of a task by its number.
*   `todo delete [index]`: Remove a task by its number.
*   `weather set [location]`: Change the monitored location.

*(Tab in command mode cycles through command history, if any exists. A minor convenience.)*

## Regarding its Purpose...

This tool is provided without warranty. It may not solve your fundamental problems or grant you immunity from surveillance. It is a dashboard. It displays data. Its primary function is to occupy terminal space with an aesthetic choice.

## Credits

*   Constructed with the aid of `rich` (providing fleeting moments of structured beauty).
*   Data acquired through `requests`, `psutil`, and the reluctant cooperation of various system libraries.
*   Inspired by flickering displays, synthetic melancholy, and the persistent need for a task list, even when the tasks are pointless.
*   The amber is non-negotiable (unless you change the theme).

```
```