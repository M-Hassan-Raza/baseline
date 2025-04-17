# Retro Terminal Dashboard
# A CLI dashboard with amber-on-black terminal aesthetic
# Requirements: pip install rich requests python-dateutil

import os
import time
import json
import requests
import datetime
from dateutil import parser
from rich.console import Console
from rich.panel import Panel
from rich.layout import Layout
from rich.text import Text
from rich.table import Table
from rich.live import Live

# Configure the console with amber-on-black theme
console = Console(color_system="standard", highlight=False)

# Custom amber color theme (approximating the vintage amber monitor look)
AMBER = "#FFBF00"
DIM_AMBER = "#CC9900"
BRIGHT_AMBER = "#FFDF00"

# Weather API key (replace with your own from weatherapi.com)
WEATHER_API_KEY = "YOUR_API_KEY"
WEATHER_LOCATION = "London"  # Change to your location

class RetroDashboard:
    def __init__(self):
        self.layout = Layout()
        self.todo_items = self.load_todos()
        self.setup_layout()

    def load_todos(self):
        """Load todo items from file or create sample ones if file doesn't exist"""
        try:
            with open('todos.json', 'r') as f:
                return json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            # Default todo items
            return [
                {"text": "Review project documentation", "done": False},
                {"text": "Debug terminal interface", "done": True},
                {"text": "Implement weather module", "done": False},
                {"text": "Optimize system performance", "done": False},
            ]

    def save_todos(self):
        """Save todo items to file"""
        with open('todos.json', 'w') as f:
            json.dump(self.todo_items, f)

    def setup_layout(self):
        """Set up the dashboard layout"""
        self.layout.split(
            Layout(name="header", size=3),
            Layout(name="main", ratio=1),
            Layout(name="footer", size=3)
        )
        
        self.layout["main"].split_row(
            Layout(name="left"),
            Layout(name="right"),
        )
        
        self.layout["left"].split(
            Layout(name="system", ratio=1),
            Layout(name="weather", ratio=1),
        )
        
        self.layout["right"].split(
            Layout(name="time", ratio=1),
            Layout(name="todos", ratio=2),
        )

    def get_header(self):
        """Create the header panel"""
        now = datetime.datetime.now()
        header_text = Text(f"RETRO TERMINAL DASHBOARD", style=AMBER, justify="center")
        subheader = Text(f"[Session: {now.strftime('%Y-%m-%d')}] [Terminal: usr/local]", 
                        style=DIM_AMBER, justify="center")
        
        full_text = Text()
        full_text.append(header_text)
        full_text.append("\n")
        full_text.append(subheader)
        
        return Panel(full_text, style=f"bold {AMBER}", border_style=AMBER)

    def get_system_info(self):
        """Create the system info panel"""
        # Get system statistics (simplified for example)
        uptime = os.popen("uptime").read().strip() if os.name != "nt" else "System running"
        mem_info = os.popen("free -h").read().strip() if os.name != "nt" else "Memory information unavailable"
        
        system_text = Text()
        system_text.append("SYSTEM STATUS\n", style=f"bold {BRIGHT_AMBER}")
        system_text.append(f"Uptime: {uptime}\n\n", style=AMBER)
        system_text.append("Memory:\n", style=AMBER)
        system_text.append(mem_info, style=DIM_AMBER)
        
        return Panel(system_text, border_style=AMBER)

    def get_weather(self):
        """Create the weather panel"""
        weather_text = Text()
        weather_text.append("WEATHER REPORT\n", style=f"bold {BRIGHT_AMBER}")
        
        try:
            # Try to get weather data from API
            if WEATHER_API_KEY == "YOUR_API_KEY":
                # Sample data if no API key provided
                weather_text.append(f"Location: {WEATHER_LOCATION}\n", style=AMBER)
                weather_text.append(f"Temperature: 22°C\n", style=AMBER)
                weather_text.append(f"Condition: Partly Cloudy\n", style=AMBER)
                weather_text.append(f"Humidity: 65%\n", style=DIM_AMBER)
                weather_text.append(f"Wind: 8 km/h\n", style=DIM_AMBER)
            else:
                # Get actual weather data
                url = f"https://api.weatherapi.com/v1/current.json?key={WEATHER_API_KEY}&q={WEATHER_LOCATION}"
                response = requests.get(url)
                data = response.json()
                
                location = data['location']['name']
                temp_c = data['current']['temp_c']
                condition = data['current']['condition']['text']
                humidity = data['current']['humidity']
                wind_kph = data['current']['wind_kph']
                
                weather_text.append(f"Location: {location}\n", style=AMBER)
                weather_text.append(f"Temperature: {temp_c}°C\n", style=AMBER)
                weather_text.append(f"Condition: {condition}\n", style=AMBER)
                weather_text.append(f"Humidity: {humidity}%\n", style=DIM_AMBER)
                weather_text.append(f"Wind: {wind_kph} km/h\n", style=DIM_AMBER)
                
        except Exception as e:
            weather_text.append(f"Weather data unavailable\n", style=AMBER)
            weather_text.append(f"ERROR: {str(e)}", style=DIM_AMBER)
            
        return Panel(weather_text, border_style=AMBER)

    def get_time_panel(self):
        """Create the time panel"""
        now = datetime.datetime.now()
        
        time_text = Text()
        time_text.append(f"{now.strftime('%H:%M:%S')}\n", style=f"bold {BRIGHT_AMBER} italic")
        time_text.append(f"{now.strftime('%A, %B %d, %Y')}\n\n", style=AMBER)
        
        # Calendar for current week
        calendar_table = Table(show_header=False, box=None, pad_edge=False, show_edge=False)
        calendar_table.add_column("Day", style=DIM_AMBER)
        calendar_table.add_column("Date", style=AMBER)
        
        # Get current week dates
        today = now.date()
        start_of_week = today - datetime.timedelta(days=today.weekday())
        
        for i in range(7):
            day = start_of_week + datetime.timedelta(days=i)
            day_name = day.strftime("%a")
            day_date = day.strftime("%d")
            
            if day == today:
                # Highlight current day
                calendar_table.add_row(f"[{BRIGHT_AMBER}]{day_name}[/{BRIGHT_AMBER}]", 
                                     f"[{BRIGHT_AMBER}]{day_date}[/{BRIGHT_AMBER}]")
            else:
                calendar_table.add_row(day_name, day_date)
        
        time_text.append(calendar_table)
        
        return Panel(time_text, title="DATE & TIME", border_style=AMBER)

    def get_todo_panel(self):
        """Create the TODO panel"""
        todo_table = Table(show_header=False, box=None, pad_edge=False, show_edge=False)
        todo_table.add_column("Status", style=BRIGHT_AMBER)
        todo_table.add_column("Task", style=AMBER)
        
        for item in self.todo_items:
            status = "[X]" if item["done"] else "[ ]"
            todo_table.add_row(status, item["text"])
            
        todo_help = Text("\n[N]ew [T]oggle [D]elete [Q]uit", style=DIM_AMBER, justify="center")
        
        todo_content = Text()
        todo_content.append(todo_table)
        todo_content.append("\n")
        todo_content.append(todo_help)
        
        return Panel(todo_content, title="TASK LIST", border_style=AMBER)

    def get_footer(self):
        """Create the footer panel"""
        footer_text = Text("Use keyboard commands to navigate the dashboard. Press '?' for help.",
                         style=DIM_AMBER, justify="center")
        return Panel(footer_text, style=AMBER, border_style=AMBER)

    def render(self):
        """Render the complete dashboard"""
        self.layout["header"].update(self.get_header())
        self.layout["system"].update(self.get_system_info())
        self.layout["weather"].update(self.get_weather())
        self.layout["time"].update(self.get_time_panel())
        self.layout["todos"].update(self.get_todo_panel())
        self.layout["footer"].update(self.get_footer())
        
        return self.layout

    def run(self):
        """Run the dashboard with live updates"""
        # Clear the screen and hide the cursor for a cleaner look
        os.system('cls' if os.name == 'nt' else 'clear')
        print("\033[?25l", end="")  # Hide cursor
        
        try:
            with Live(self.render(), refresh_per_second=1, screen=True) as live:
                while True:
                    live.update(self.render())
                    time.sleep(1)
        except KeyboardInterrupt:
            # Show cursor again before exiting
            print("\033[?25h", end="")
            print("Dashboard terminated.")

if __name__ == "__main__":
    # Set terminal title
    print("\033]0;Retro Terminal Dashboard\007", end="")
    
    # Apply terminal styling for amber-on-black effect
    # Note: This may not work in all terminals
    print("\033]10;#FFBF00\007", end="")  # Set text color to amber
    print("\033]11;#000000\007", end="")  # Set background to black
    
    # Run the dashboard
    dashboard = RetroDashboard()
    dashboard.run()