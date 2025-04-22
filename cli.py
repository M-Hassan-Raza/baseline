# Baseline Terminal Dashboard
# A CLI dashboard with amber-on-black terminal aesthetic
# Requirements: pip install rich requests python-dateutil psutil python-dotenv

import os
import time
import json
import requests
import datetime
import psutil
import platform
import socket
import subprocess
from dateutil import parser
from rich.console import Console
from rich.panel import Panel
from rich.layout import Layout
from rich.text import Text
from rich.table import Table
from rich.live import Live
from rich import box
from pathlib import Path
from dotenv import load_dotenv

# Load environment variables from .env file
env_path = Path(__file__).parent / ".env"
load_dotenv(dotenv_path=env_path)

console = Console(color_system="standard", highlight=False)

# Custom amber color theme
AMBER = "#FFBF00"
DIM_AMBER = "#CC9900"
BRIGHT_AMBER = "#FFDF00"

WEATHER_API_KEY = os.getenv("WEATHER_API_KEY", "YOUR_API_KEY")
WEATHER_LOCATION = os.getenv("WEATHER_LOCATION", "Lahore")

# Get number of CPU cores for normalizing CPU percentages in process list
CPU_COUNT = psutil.cpu_count() or 1


class Baseline:
    def __init__(self):
        self.layout = Layout()
        self.todo_items = self.load_todos()
        self.commands = []
        self.command_history = []
        self.setup_layout()
        self.notifications = []
        self.current_focus = "dashboard"  # dashboard, todo, command
        self.command_input = ""
        self.todo_input_mode = False
        self.new_todo_text = ""
        self.tab_index = 0

        # Create data directory if it doesn't exist
        self.data_dir = Path.home() / ".baseline"
        self.data_dir.mkdir(exist_ok=True)

        self.system_history = self.load_system_history()
        self.last_update = time.time()

        self.last_net_io = psutil.net_io_counters()
        self.last_net_time = time.time()

        self.theme = os.getenv("THEME", "amber")
        self.themes = {
            "amber": {"main": "#FFBF00", "dim": "#CC9900", "bright": "#FFDF00"},
            "green": {"main": "#00FF00", "dim": "#009900", "bright": "#CCFFCC"},
            "blue": {"main": "#00BFFF", "dim": "#0099CC", "bright": "#99CCFF"},
        }

        self.shortcuts = {
            "n": "New TODO",
            "t": "Toggle TODO",
            "d": "Delete TODO",
            "p": "Change Priority",
            "q": "Quit",
            ":": "Command Mode",
            "?": "Show Help",
            "Tab": "Switch Panels",
            "F5": "Refresh Data",
        }

    def load_todos(self):
        """Load todo items from file or create sample ones if file doesn't exist"""
        todo_file = Path.home() / ".baseline" / "todos.json"
        try:
            if todo_file.exists():
                with open(todo_file, "r") as f:
                    return json.load(f)
            else:
                # Default todo items if file not found
                return [
                    {
                        "text": "Review project documentation",
                        "done": False,
                        "priority": "medium",
                    },
                    {
                        "text": "Debug terminal interface",
                        "done": True,
                        "priority": "high",
                    },
                    {
                        "text": "Implement weather module",
                        "done": False,
                        "priority": "medium",
                    },
                    {
                        "text": "Optimize system performance",
                        "done": False,
                        "priority": "low",
                    },
                ]
        except (json.JSONDecodeError, Exception) as e:
            self.add_notification(f"Error loading todos: {str(e)}", "error")
            return []

    def save_todos(self):
        """Save todo items to file"""
        todo_file = Path.home() / ".baseline" / "todos.json"
        try:
            with open(todo_file, "w") as f:
                json.dump(self.todo_items, f)
        except Exception as e:
            self.add_notification(f"Error saving todos: {str(e)}", "error")

    def load_system_history(self):
        """Load system monitoring history or create a new one"""
        history_file = self.data_dir / "system_history.json"
        try:
            if history_file.exists():
                with open(history_file, "r") as f:
                    return json.load(f)
            else:
                # Create empty history structure
                return {
                    "cpu": [],
                    "memory": [],
                    "timestamps": [],
                    "network_in": [],
                    "network_out": [],
                }
        except (json.JSONDecodeError, Exception) as e:
            self.add_notification(f"Error loading system history: {str(e)}", "error")
            return {
                "cpu": [],
                "memory": [],
                "timestamps": [],
                "network_in": [],
                "network_out": [],
            }

    def save_system_history(self):
        """Save system monitoring history, keeping only the last 60 data points"""
        history_file = self.data_dir / "system_history.json"
        try:
            for key in self.system_history:
                if len(self.system_history[key]) > 60:
                    self.system_history[key] = self.system_history[key][-60:]

            with open(history_file, "w") as f:
                json.dump(self.system_history, f)
        except Exception as e:
            self.add_notification(f"Error saving system history: {str(e)}", "error")

    def update_system_history(self):
        """Update system monitoring history, throttled to run every 5 seconds"""
        current_time = time.time()
        if current_time - self.last_update < 5:
            return
        self.last_update = current_time

        cpu_percent = psutil.cpu_percent()
        memory_percent = psutil.virtual_memory().percent
        net_io = psutil.net_io_counters()
        net_in = net_io.bytes_recv
        net_out = net_io.bytes_sent

        timestamp = datetime.datetime.now().strftime("%H:%M:%S")
        self.system_history["cpu"].append(cpu_percent)
        self.system_history["memory"].append(memory_percent)
        self.system_history["timestamps"].append(timestamp)
        self.system_history["network_in"].append(net_in)
        self.system_history["network_out"].append(net_out)

        self.save_system_history()

    def setup_layout(self):
        """Set up the dashboard layout using Rich"""
        self.layout.split(
            Layout(name="header", size=3),
            Layout(name="main", ratio=1),
            Layout(name="footer", size=3),
        )

        self.layout["main"].split_row(
            Layout(name="left", ratio=1),
            Layout(name="right", ratio=1),
        )

        self.layout["left"].split(
            Layout(name="system", ratio=1),
            Layout(name="weather", ratio=1),
        )

        self.layout["right"].split(
            Layout(name="time", ratio=1),
            Layout(name="todos", ratio=2),
        )

    def add_notification(self, message, type="info"):
        """Add a notification, keeping only the last 5"""
        self.notifications.append(
            {
                "message": message,
                "type": type,
                "time": datetime.datetime.now().strftime("%H:%M:%S"),
            }
        )
        if len(self.notifications) > 5:
            self.notifications = self.notifications[-5:]

    def get_header(self):
        """Create the header panel"""
        now = datetime.datetime.now()
        header_text = Text(f"BASELINE", style=AMBER, justify="center")
        hostname = socket.gethostname()
        username = (
            os.getlogin() if hasattr(os, "getlogin") else os.environ.get("USER", "user")
        )
        subheader = Text(
            f"[Session: {now.strftime('%Y-%m-%d')}] [Terminal: {username}@{hostname}]",
            style=DIM_AMBER,
            justify="center",
        )

        full_text = Text()
        full_text.append(header_text)
        full_text.append("\n")
        full_text.append(subheader)

        return Panel(full_text, style=f"bold {AMBER}", border_style=AMBER)

    def get_system_info(self):
        """Create the system info panel with graphs"""
        self.update_system_history()

        cpu_percent = psutil.cpu_percent()
        memory = psutil.virtual_memory()
        disk = psutil.disk_usage("/")

        system_text = Text()
        system_text.append("SYSTEM STATUS\n", style=f"bold {BRIGHT_AMBER}")
        system_text.append(f"Host: {platform.node()}\n", style=AMBER)
        system_text.append(
            f"OS: {platform.system()} {platform.release()}\n", style=AMBER
        )
        uptime = datetime.timedelta(seconds=int(time.time() - psutil.boot_time()))
        system_text.append(f"Uptime: {uptime}\n", style=AMBER)

        system_text.append("\nCPU: ", style=AMBER)
        cpu_bar = self.create_bar(cpu_percent)
        system_text.append(f"{cpu_bar} {cpu_percent}%\n", style=BRIGHT_AMBER)

        system_text.append("MEM: ", style=AMBER)
        mem_bar = self.create_bar(memory.percent)
        system_text.append(f"{mem_bar} {memory.percent}%\n", style=BRIGHT_AMBER)

        system_text.append("DSK: ", style=AMBER)
        disk_bar = self.create_bar(disk.percent)
        system_text.append(f"{disk_bar} {disk.percent}%\n", style=BRIGHT_AMBER)

        # Network I/O throughput calculation
        try:
            current_net_io = psutil.net_io_counters()
            current_time = time.time()
            time_diff = current_time - self.last_net_time

            if time_diff > 0:
                rx_rate = (
                    (current_net_io.bytes_recv - self.last_net_io.bytes_recv)
                    / time_diff
                    / 1024  # KB/s
                )
                tx_rate = (
                    (current_net_io.bytes_sent - self.last_net_io.bytes_sent)
                    / time_diff
                    / 1024  # KB/s
                )
                system_text.append(
                    f"NET: ↓ {rx_rate:.1f} KB/s ↑ {tx_rate:.1f} KB/s\n", style=AMBER
                )
                self.last_net_io = current_net_io
                self.last_net_time = current_time
        except Exception as e:
            system_text.append(f"NET: Error measuring network I/O\n", style=DIM_AMBER)

        # Running processes (top 3 by CPU, normalized)
        system_text.append("\nTOP PROCESSES:\n", style=AMBER)
        processes = []
        for proc in psutil.process_iter(["pid", "name", "cpu_percent"]):
            try:
                # Normalize CPU percentage by dividing by the number of cores
                proc_info = proc.info
                proc_info["cpu_percent"] = proc_info["cpu_percent"] / CPU_COUNT
                processes.append(proc_info)
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                pass

        top_processes = sorted(processes, key=lambda x: x["cpu_percent"], reverse=True)[
            :3
        ]
        for proc in top_processes:
            system_text.append(f"{proc['name'][:15]:<15} ", style=DIM_AMBER)
            system_text.append(f"CPU: {proc['cpu_percent']:.1f}%\n", style=AMBER)

        return Panel(system_text, border_style=AMBER)

    def create_bar(self, percentage, width=15):
        """Create a text-based progress bar"""
        filled_width = int(width * percentage / 100)
        bar = "[" + "█" * filled_width + "░" * (width - filled_width) + "]"
        return bar

    def get_weather(self):
        """Create the weather panel"""
        weather_text = Text()
        weather_text.append("WEATHER REPORT\n", style=f"bold {BRIGHT_AMBER}")

        try:
            if WEATHER_API_KEY == "YOUR_API_KEY":
                # Sample data if no API key provided
                weather_text.append(f"Location: {WEATHER_LOCATION}\n", style=AMBER)
                weather_text.append(f"Temperature: 22°C\n", style=AMBER)
                weather_text.append(f"Condition: Partly Cloudy\n", style=AMBER)
                weather_text.append(f"Humidity: 65%\n", style=DIM_AMBER)
                weather_text.append(f"Wind: 8 km/h\n", style=DIM_AMBER)

                weather_text.append(
                    "\nSet WEATHER_API_KEY in .env file\n", style=DIM_AMBER
                )

                weather_text.append("\n    \\  /\n", style=BRIGHT_AMBER)
                weather_text.append('  _ /"".-.    \n', style=BRIGHT_AMBER)
                weather_text.append("    \\_(   ).  \n", style=BRIGHT_AMBER)
                weather_text.append("    /(___(__)  \n", style=BRIGHT_AMBER)
            else:
                # Get actual weather data
                url = f"https://api.weatherapi.com/v1/current.json?key={WEATHER_API_KEY}&q={WEATHER_LOCATION}"
                response = requests.get(url)
                data = response.json()

                location = data["location"]["name"]
                temp_c = data["current"]["temp_c"]
                condition = data["current"]["condition"]["text"]
                humidity = data["current"]["humidity"]
                wind_kph = data["current"]["wind_kph"]

                weather_text.append(f"Location: {location}\n", style=AMBER)
                weather_text.append(f"Temperature: {temp_c}°C\n", style=AMBER)
                weather_text.append(f"Condition: {condition}\n", style=AMBER)
                weather_text.append(f"Humidity: {humidity}%\n", style=DIM_AMBER)
                weather_text.append(f"Wind: {wind_kph} km/h\n", style=DIM_AMBER)

        except Exception as e:
            weather_text.append(f"Weather data unavailable\n", style=AMBER)
            weather_text.append(f"ERROR: {str(e)}\n", style=DIM_AMBER)

        # Add forecast (currently static sample data)
        weather_text.append("\nFORECAST:\n", style=AMBER)
        hours = ["06:00", "12:00", "18:00", "00:00"]
        temps = ["18°C", "22°C", "20°C", "16°C"]

        for i, (hour, temp) in enumerate(zip(hours, temps)):
            weather_text.append(f"{hour}: {temp}\n", style=DIM_AMBER)

        return Panel(weather_text, border_style=AMBER)

    def get_time_panel(self):
        """Create the time panel with calendar"""
        now = datetime.datetime.now()

        time_text = Text()
        time_text.append(
            f"{now.strftime('%H:%M:%S')}\n", style=f"bold {BRIGHT_AMBER} italic"
        )
        time_text.append(f"{now.strftime('%A, %B %d, %Y')}\n\n", style=AMBER)

        time_text.append("     CALENDAR     \n", style=AMBER)
        time_text.append("Mo Tu We Th Fr Sa Su\n", style=DIM_AMBER)

        import calendar

        cal = calendar.monthcalendar(now.year, now.month)

        current_week = None
        for i, week in enumerate(cal):
            if now.day in week:
                current_week = i
                break

        for week in cal:
            week_text = ""
            for day in week:
                if day == 0:
                    week_text += "   "
                elif day == now.day:
                    week_text += f"{day:2d}*"  # Highlight current day
                else:
                    week_text += f"{day:2d} "
            time_text.append(
                week_text + "\n",
                style=AMBER if week == cal[current_week] else DIM_AMBER,
            )

        # Add upcoming appointments (currently static sample data)
        time_text.append("\nUPCOMING:\n", style=AMBER)
        events = [
            {"time": "14:00", "name": "Team Meeting"},
            {"time": "16:30", "name": "Project Review"},
            {"time": "Tomorrow", "name": "Deadline: Report"},
        ]

        for event in events:
            time_text.append(f"{event['time']}: ", style=DIM_AMBER)
            time_text.append(f"{event['name']}\n", style=AMBER)

        return Panel(time_text, border_style=AMBER)

    def get_todo_panel(self):
        """Create the TODO panel"""
        sorted_todos = sorted(
            self.todo_items,
            key=lambda x: {"high": 0, "medium": 1, "low": 2}.get(
                x.get("priority", "medium"), 1
            ),
        )

        todo_text = Text()

        if self.todo_input_mode:
            todo_text.append("NEW TASK:\n", style=BRIGHT_AMBER)
            todo_text.append(f"{self.new_todo_text}", style=AMBER)
            todo_text.append("_", style=BRIGHT_AMBER)  # Cursor simulation
            todo_text.append("\n\n", style=AMBER)

        for i, item in enumerate(sorted_todos):
            priority = item.get("priority", "medium")
            if priority == "high":
                priority_char = "!"
                priority_style = BRIGHT_AMBER
            elif priority == "medium":
                priority_char = "o"
                priority_style = AMBER
            else:  # low
                priority_char = "-"
                priority_style = DIM_AMBER

            todo_text.append(f"{i+1:2d} ", style=DIM_AMBER)
            todo_text.append(f"[{priority_char}] ", style=priority_style)

            status = "[X]" if item["done"] else "[ ]"
            todo_text.append(
                f"{status} ", style=BRIGHT_AMBER if item["done"] else AMBER
            )

            if item["done"]:
                # Simulate strikethrough with Unicode combining character
                task_text = "".join([c + "\u0336" for c in item["text"]])
                todo_text.append(f"{task_text}\n", style=DIM_AMBER)
            else:
                todo_text.append(f"{item['text']}\n", style=AMBER)

        todo_help = Text(
            "\n[N]ew [T]oggle [D]elete [P]riority [Q]uit",
            style=DIM_AMBER,
            justify="center",
        )
        todo_text.append("\n")
        todo_text.append(todo_help)

        return Panel(todo_text, title="TASK LIST", border_style=AMBER)

    def get_footer(self):
        """Create the footer panel with notifications or command input"""
        footer_text = Text()

        if self.current_focus == "command":
            footer_text.append("> ", style=BRIGHT_AMBER)
            footer_text.append(f"{self.command_input}", style=AMBER)
            footer_text.append("_", style=BRIGHT_AMBER)  # Cursor simulation
        else:
            if self.notifications:
                latest = self.notifications[-1]
                notification_style = {
                    "info": AMBER,
                    "error": "#FF5555",
                    "success": "#55FF55",
                }.get(latest["type"], AMBER)
                footer_text.append(f"[{latest['time']}] ", style=DIM_AMBER)
                footer_text.append(f"{latest['message']}", style=notification_style)
            else:
                footer_text.append(
                    "Press ':' to enter command mode, '?' for help", style=DIM_AMBER
                )

        return Panel(footer_text, style=AMBER, border_style=AMBER)

    def render(self):
        """Render the complete dashboard layout"""
        self.layout["header"].update(self.get_header())
        self.layout["system"].update(self.get_system_info())
        self.layout["weather"].update(self.get_weather())
        self.layout["time"].update(self.get_time_panel())
        self.layout["todos"].update(self.get_todo_panel())
        self.layout["footer"].update(self.get_footer())
        return self.layout

    def process_command(self, command):
        """Process command input from the footer"""
        command = command.strip().lower()
        if not command:
            return

        self.command_history.append(command)
        if len(self.command_history) > 20:
            self.command_history = self.command_history[-20:]

        if command == "help" or command == "?":
            self.add_notification(
                "Commands: help, todo, weather, clear, exit, theme, shortcut", "info"
            )
        elif command == "exit" or command == "quit":
            raise KeyboardInterrupt()
        elif command == "clear":
            self.notifications = []
            self.add_notification("Notifications cleared", "success")
        elif command == "shortcut":
            shortcuts_str = ", ".join([f"{k}:{v}" for k, v in self.shortcuts.items()])
            self.add_notification(f"Shortcuts: {shortcuts_str}", "info")
        elif command.startswith("theme "):
            theme_name = command[6:].strip()
            if theme_name in self.themes:
                self.theme = theme_name
                self.add_notification(f"Theme changed to {theme_name}", "success")
            else:
                self.add_notification(
                    f"Unknown theme: {theme_name}. Available: {', '.join(self.themes.keys())}",
                    "error",
                )
        elif command.startswith("todo add "):
            text = command[9:].strip()
            if text:
                self.todo_items.append(
                    {"text": text, "done": False, "priority": "medium"}
                )
                self.save_todos()
                self.add_notification(f"Added todo: {text}", "success")
        elif command.startswith("todo toggle "):
            try:
                index = int(command[12:].strip()) - 1
                if 0 <= index < len(self.todo_items):
                    self.todo_items[index]["done"] = not self.todo_items[index]["done"]
                    self.save_todos()
                    self.add_notification(f"Toggled todo #{index+1}", "success")
                else:
                    self.add_notification(f"Invalid todo index: {index+1}", "error")
            except ValueError:
                self.add_notification("Invalid todo index", "error")
        elif command.startswith("todo delete "):
            try:
                # Corrected typo: .trip() -> .strip()
                index = int(command[12:].strip()) - 1
                if 0 <= index < len(self.todo_items):
                    deleted = self.todo_items.pop(index)
                    self.save_todos()
                    self.add_notification(f"Deleted todo: {deleted['text']}", "success")
                else:
                    self.add_notification(f"Invalid todo index: {index+1}", "error")
            except ValueError:
                self.add_notification("Invalid todo index", "error")
        elif command.startswith("weather set "):
            location = command[12:].strip()
            global WEATHER_LOCATION
            WEATHER_LOCATION = location
            self.add_notification(f"Weather location set to: {location}", "success")
        else:
            self.add_notification(f"Unknown command: {command}", "error")

        self.command_input = ""
        self.current_focus = "dashboard"

    def process_key(self, key):
        """Process a key press based on the current focus"""
        # Command mode input handling
        if self.current_focus == "command":
            if key == "\r" or key == "\n":  # Handle Enter key variations
                self.process_command(self.command_input)
            elif key == "\x1b":  # Escape key
                self.command_input = ""
                self.current_focus = "dashboard"
            elif key == "\x08" or key == "\x7f":  # Backspace/Delete keys
                self.command_input = self.command_input[:-1]
            elif key == "\t" and self.command_history:  # Tab for history
                self.tab_index = (self.tab_index + 1) % len(self.command_history)
                self.command_input = self.command_history[self.tab_index]
            elif key.isprintable():  # Only add printable characters
                self.command_input += key

        # Todo input mode handling
        elif self.todo_input_mode:
            if key == "\r" or key == "\n":  # Enter key
                if self.new_todo_text:
                    self.todo_items.append(
                        {
                            "text": self.new_todo_text,
                            "done": False,
                            "priority": "medium",
                        }
                    )
                    self.save_todos()
                    self.add_notification(
                        f"Added todo: {self.new_todo_text}", "success"
                    )
                self.new_todo_text = ""
                self.todo_input_mode = False
            elif key == "\x1b":  # Escape key
                self.new_todo_text = ""
                self.todo_input_mode = False
            elif key == "\x08" or key == "\x7f":  # Backspace/Delete keys
                self.new_todo_text = self.new_todo_text[:-1]
            elif key.isprintable():  # Only add printable characters
                self.new_todo_text += key

        # Dashboard mode - global shortcuts
        else:
            if key == ":":
                self.current_focus = "command"
            elif key == "n":
                self.todo_input_mode = True
            elif key == "t" and self.todo_items:
                # Toggle the first uncompleted todo
                for i, item in enumerate(self.todo_items):
                    if not item["done"]:
                        self.todo_items[i]["done"] = True
                        self.save_todos()
                        self.add_notification(f"Completed: {item['text']}", "success")
                        break
            elif key == "d" and self.todo_items:
                # Delete the first completed todo
                for i, item in enumerate(self.todo_items):
                    if item["done"]:
                        deleted = self.todo_items.pop(i)
                        self.save_todos()
                        self.add_notification(f"Deleted: {deleted['text']}", "success")
                        break
            elif key == "p" and self.todo_items:
                # Cycle priority of the first uncompleted todo
                priorities = ["low", "medium", "high"]
                for i, item in enumerate(self.todo_items):
                    if not item["done"]:
                        current = item.get("priority", "medium")
                        idx = (priorities.index(current) + 1) % len(priorities)
                        self.todo_items[i]["priority"] = priorities[idx]
                        self.save_todos()
                        self.add_notification(
                            f"Priority set to {priorities[idx]}", "success"
                        )
                        break
            elif key == "q":
                raise KeyboardInterrupt()
            elif key == "?":
                self.add_notification(
                    "Keys: n(new), t(toggle), d(delete), p(priority), q(quit), :(command)",
                    "info",
                )

    def run(self):
        """Run the dashboard main loop with live updates and keyboard input"""
        os.system("cls" if os.name == "nt" else "clear")
        print("\033[?25l", end="")  # Hide cursor

        try:
            self.add_notification("Welcome to Baseline", "info")

            # Platform-specific keyboard input handling
            if os.name == "nt":
                import msvcrt

                with Live(self.render(), refresh_per_second=4, screen=True) as live:
                    while True:
                        if msvcrt.kbhit():
                            # Decode bytes to string, handle potential errors
                            try:
                                key = msvcrt.getch().decode("utf-8", errors="ignore")
                                # Map special keys if needed (e.g., Enter, Backspace)
                                if key == "\r":
                                    key = "enter"  # Map Enter
                                elif key == "\x08":
                                    key = "backspace"  # Map Backspace
                                elif key == "\x1b":
                                    key = "escape"  # Map Escape
                                elif key == "\t":
                                    key = "tab"  # Map Tab
                                # Add other mappings as necessary
                                self.process_key(key)
                            except UnicodeDecodeError:
                                pass  # Ignore undecodable bytes
                        live.update(self.render())
                        time.sleep(0.1)
            else:
                import curses

                stdscr = curses.initscr()
                curses.noecho()
                curses.cbreak()
                stdscr.nodelay(True)  # Non-blocking input
                stdscr.keypad(True)  # Enable special keys like arrows, backspace

                with Live(self.render(), refresh_per_second=4, screen=True) as live:
                    while True:
                        try:
                            key_code = stdscr.getch()  # Use getch for key codes
                            if key_code != -1:
                                key = curses.keyname(key_code).decode("utf-8")
                                # Map curses key names to expected values
                                if key == "^M" or key == "\n":
                                    key = "enter"
                                elif key == "^?" or key == "KEY_BACKSPACE":
                                    key = "backspace"
                                elif key == "^[":
                                    key = "escape"  # Often Escape sequence start
                                elif key == "^I":
                                    key = "tab"
                                # Add more mappings if needed (e.g., KEY_UP, KEY_DOWN)
                                self.process_key(key)
                        except curses.error:
                            pass  # No key pressed
                        live.update(self.render())
                        time.sleep(0.1)
        except KeyboardInterrupt:
            print("\033[?25h", end="")
            print("Dashboard terminated.")
        finally:
            print("\033[?25h", end="")  # Ensure cursor is visible
            if os.name != "nt":
                # Clean up curses environment
                curses.nocbreak()
                stdscr.keypad(False)
                curses.echo()
                curses.endwin()


if __name__ == "__main__":
    # Set terminal title (may not work on all terminals)
    print("\033]0;Baseline\007", end="")
    # Apply terminal styling (may not work on all terminals)
    print("\033]10;#FFBF00\007", end="")
    print("\033]11;#000000\007", end="")

    dashboard = Baseline()
    dashboard.run()
