-- A card may have a start date, where its bar begins on the board's Gantt chart.
ALTER TABLE cards ADD COLUMN start_date date;
