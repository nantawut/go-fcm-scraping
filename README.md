# FIFA Career Mode Player Scraper

This is a concurrent web scraper written in Go designed to scout for high-potential young players from fifacm.com. It efficiently fetches data for multiple teams, filters players based on configurable criteria (like minimum potential and growth), and saves the results to a clean JSON file.

## Features

-   **Concurrent Scraping**: Utilizes goroutines and a semaphore to process multiple teams in parallel, significantly speeding up the data collection process.
-   **Configurable Filtering**: Easily set minimum potential and growth values to find the exact type of players you're looking for.
-   **Rate-Limit Avoidance**: Implements randomized delays and rotates `User-Agent` headers for each request to mimic human behavior and avoid being blocked.
-   **Robust Error Handling**: Gracefully handles HTTP errors and network issues for individual teams without crashing the entire process.
-   **Clean JSON Output**: Saves the final list of players as a well-formatted, valid JSON array, perfect for use in other applications or for easy viewing.
-   **Encapsulated & Performant**: The scraper's logic is encapsulated in a `Scraper` struct, and regular expressions are pre-compiled for better performance.

## How to Run

1.  **Prerequisites**: Ensure you have Go installed on your system.
2.  **Run the Scraper**: Navigate to the project directory in your terminal and run the following command:
3.  **Check the Output**: The script will start logging its progress to the console. Once complete, it will generate a file named `high_potential_players.json` in the same directory containing the list of scouted players.

    
