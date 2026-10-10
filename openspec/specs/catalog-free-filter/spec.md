# catalog-free-filter Specification

## Purpose

The catalog List view's "Free only" filter: which images it shows, and how the first free-tier check, which can take about 30 seconds, reports its progress instead of a bare loading message.

FF-1 defines the filter's result; FF-2 to FF-5 how the check runs and reports progress; FF-6 and FF-7 changes and errors during the check; FF-8 layout, theme and security policy. Live figures were measured on 2026-10-10: 3,171 images, 59 free, checked in 13 batches of 250 that took about 2.2 seconds each (about 28 seconds in all); once cached, each batch answered in under 5 milliseconds.

## Requirements

### Requirement: FF-1 Free images only
When "Free only" is ticked, the List view SHALL show only the free-tier images that match the search, paginated as when it is not ticked, with their count.

#### Scenario: FF-1.1 Whole catalog
- **WHEN** "Free only" is ticked with an empty search and the check has finished
- **THEN** the list says "59 images" and shows only images with the Free badge

#### Scenario: FF-1.2 Unticked
- **WHEN** "Free only" is not ticked
- **THEN** the List view requests one page of images directly, with no free-tier check and no progress bar

### Requirement: FF-2 Batched free-tier check
When "Free only" is ticked, the List view SHALL first check the free-tier status of every image matching the search by requesting the image list in batches of at most 250 images, one batch at a time, each starting after the previous one finished. Only when every batch has returned SHALL it request the free-only list.

#### Scenario: FF-2.1 Whole catalog
- **WHEN** "Free only" is ticked with an empty search
- **THEN** 13 image list requests are made with a limit of 250 and offsets 0, 250, 500 … 3,000, each after the previous one returned
- **AND** then one free-only request is made for the first page

#### Scenario: FF-2.2 With a search
- **WHEN** "Free only" is ticked with the search `python`, which matches 11 images
- **THEN** one batch request is made for those 11 images, followed by the free-only request

### Requirement: FF-3 Progress bar
While the check runs, the List view SHALL show a progress bar whose length is the share of matching images checked so far, labeled in text with the count checked and the total, and an estimate of the time left (FF-4). The bar SHALL be exposed to assistive technology as a progress bar with its current value, maximum and text, and its label SHALL be announced politely as it changes.

#### Scenario: FF-3.1 First batch returned
- **WHEN** the first batch of 250 has returned and the whole catalog of 3,171 images is being checked
- **THEN** the bar is filled to 250/3,171 of its length and the label begins "Checking free-tier status: 250 of 3,171 images ·", followed by the time left (FF-4)

#### Scenario: FF-3.2 Before any batch returns
- **WHEN** the check has run for more than a second and no batch has returned
- **THEN** the bar is empty and the label reads "Checking free-tier status: 0 of 3,171 images · about 30 seconds left" (3,171 images at the assumed 9 ms each is 28.5 seconds, rounded up to 30)

#### Scenario: FF-3.3 Screen readers
- **WHEN** the bar is shown after 1,000 of 3,171 images are checked
- **THEN** it has the progress bar role with a current value of 1,000, a maximum of 3,171 and a text value matching its visible label

### Requirement: FF-4 Time left
The label SHALL estimate the time left for the images not yet checked: before any batch has returned, at an assumed 9 ms per image (the rate measured on 2026-10-10); after that, at the average time per image checked so far. The estimate SHALL be rounded up to the next 5 seconds and shown as "about N seconds left", or as "a few seconds left" when under 5 seconds.

#### Scenario: FF-4.1 Mid-check estimate
- **WHEN** 1,000 of 3,171 images were checked in 8.8 seconds
- **THEN** the label ends "about 20 seconds left" (2,171 images at 8.8 ms each is 19.1 seconds, rounded up to 20)

#### Scenario: FF-4.2 Nearly done
- **WHEN** 3,000 of 3,171 images were checked in 26.4 seconds
- **THEN** the label ends "a few seconds left" (171 images at 8.8 ms each is 1.5 seconds)

### Requirement: FF-5 No bar for a fast check
The progress bar SHALL appear only if the check is still running one second after it started. A check that finishes sooner, such as when the server has the statuses cached, SHALL show the usual loading message until the list appears.

#### Scenario: FF-5.1 Cached statuses
- **WHEN** "Free only" is ticked and every batch returns within one second in total
- **THEN** no progress bar is ever shown and the free images appear

### Requirement: FF-6 Changes during the check
If "Free only" is unticked or the page is left during the check, the List view SHALL request no further batches. If the search changes during the check, it SHALL request no further batches for the old search and SHALL start the check for the new search.

#### Scenario: FF-6.1 Unticked mid-check
- **WHEN** "Free only" is unticked after 3 of 13 batches have returned
- **THEN** no further batch is requested and the unfiltered list is shown

#### Scenario: FF-6.2 Search changed mid-check
- **WHEN** the search changes to `python` while the whole catalog is being checked
- **THEN** no further batch for the empty search is requested, and the check restarts for the 11 images matching `python`

### Requirement: FF-7 A batch fails
If a batch request fails, the List view SHALL keep the progress reached, show the error with a retry option, and on retry SHALL request only the batches not yet returned.

#### Scenario: FF-7.1 Retry resumes
- **WHEN** the batch at offset 1,000 fails after 4 batches returned, and the user retries
- **THEN** the bar still shows 1,000 of 3,171 checked while the error is shown, and the retry requests offsets 1,000 onward, not 0 to 750 again

### Requirement: FF-8 Layout, theme and security policy
The progress bar and its label SHALL be usable at 375 px wide and in light and dark mode, and SHALL cause no Content Security Policy violations.

#### Scenario: FF-8.1 Phone width in dark mode
- **WHEN** the check is shown at 375 px wide in dark mode
- **THEN** the bar and label fit without horizontal scrolling, the filled and unfilled parts are distinguishable, and the console shows no Content Security Policy violations
