# Week 3: Frontend Landing Page and Routing

## Objective

The objective for Week 3 was to develop the initial user-facing interface of the Research Agent and establish the frontend navigation structure for the application.

## Landing Page

A dedicated landing page was developed under `src/pages/Home.tsx`.

The page provides the initial entry point for users and includes:

- Research Agent branding
- An introduction to the research and data synthesis functionality
- A company name or stock ticker input
- An Analyze button
- Example company/ticker queries such as TCS, INFY.NS, AAPL, and RELIANCE.NS

The landing page was designed to provide a simple interface through which a user can start a research query.

## Frontend Routing

React Router was introduced into the application structure to support navigation between different pages.

The main application was updated to use `BrowserRouter`, `Routes`, and `Route`. The home page was configured as the initial `/` route.

The company search form was also prepared to navigate to a company-specific report URL using the company name or ticker entered by the user.

The report page itself will be implemented in a later stage once the agent workflow and report generation functionality are completed.

## Styling

The frontend styling was updated through `src/index.css` along with the existing Tailwind-based styling approach.

The landing page was designed with a clean dark interface and highlighted actions to make the research workflow easy to understand.

## Frontend Configuration

The Vite frontend configuration and package dependencies were updated as part of the new frontend functionality.

The project continued to use React, TypeScript, and Vite as the primary frontend technologies.

## Key Learnings

- Learned how to structure a React application using separate page components.
- Improved understanding of client-side routing using React Router.
- Learned how user input can be used to construct dynamic navigation paths.
- Understood how frontend navigation can be prepared before the corresponding backend/report functionality is completed.
- Improved understanding of organizing UI code into reusable and maintainable components.

## Outcome

By the end of Week 3, the Research Agent frontend had a functional landing page and an initial routing structure.

The frontend now provides a clear entry point for users to enter a company or stock ticker and prepares the application for the research and report-generation workflow that will be developed in subsequent weeks.