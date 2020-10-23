requirement rit en park_events registreren.

Verschillende strategiën voor importeren.

Als alle fiets opeens uit een feed verdwijnen wordt er vanuitgegaan dat er een storing is bij de aanbieder.
1. Optionele waarschuwing stuur een email uit met een waarschuwing naar info@deelfietsdashboard.nl
2. Wat doen we als een aanbieder echt stopt met fietsen aanbieden? (handmatige actie?)

clean:
Data zoals van Keobike, Hely en Donkey. De locatie kan alleen worden geupdate door operator of gebruikers via Mobiele app.

gps:
Data die op GPS gebaseerd (voertuigen hebben GPS) zoals we die van Check, Felyx en Gosharing krijgen.

Als een fiets verschijnt (dat die niet in de gbfs feed zit en dan wel in de gbfs feed) in de feed wordt het beschouwd als een check_in. 
Als een fiets verdwijnt (dat de fiets in de gbfs feed zit en dan niet meer in de gbfs feed zit) uit de feed wordt het beschouwd als een check_out.
Als een fiets binnen vijftien (?) minuten weer ongeveer op dezelfde plaats (100m) terugkeert wordt er vanuit gegaan dat de fiets niet gebruikt is geweest (waarschijnlijk reserving of iets dergelijks).  
Als een fiets in de GBFS blijft, maar de afstand is groter dan 500m een nieuw park_event maar geen rit.
kleiner dan 500m GPS locatie actualiseren. 



Als een fiets verschijnt in de feed wordt het beschouwd als een check_in. 
Als een fiets verdwijnt uit de feed wordt het beschouwd als een check_out.
Als een fiets binnen vijftien (?) minuten weer ongeveer (100m) op dezelfde plaats terugkeert wordt er vanuit gegaan dat de fiets niet gebruikt is geweest (waarschijnlijk reserving of iets dergelijks).  
Als een fiets in de GBFS blijft, maar de afstand is groter dan 500m een nieuw park_event maar geen rit.
kleiner dan 500 GPS locatie actualiseren. 

flickbike:
Strategie waar fiets eerst terug komt in de feed en daarna de locatie pas geupdate wordt.

Als een fiets verschijnt in de feed wordt het beschouwd als een check_in. 
Als een fiets binnen de GBFS blijft binnen 5 minuten na de check_in wordt de locatie van het gecreerde park_event gewijzigd en de eind locatie van de trip ook gewijzigd. 
Als een fiets verdwijnt uit de feed wordt het beschouwd als een check_out.  
Als een fiets in de GBFS blijft, maar de afstand is groter dan 500m een nieuw park_event maar geen rit.
kleiner dan 500 GPS locatie actualiseren. 

rotating:
checkout_forever

opschonen:
komt wel een keer, als roterende id's goed gaan dan komt het waarschijnlijk wel goed.

# 's Nachts opschonen.
's nachts opschonen < 0 minutent te huur events.
< 0 minuten verhuur events. 
heel veel kort huur events achter elkaar, met steeds andere gps locaties. 
aantal fietsen daal meer dan met x% (alarmbellen)af laat gaan. 
check hoe het gaat met Zwolse deelfiets.


Basis: 
Initiele check_in 
1. check of voertuig al bestaat
a. ja 
als park_event open dan doe niks
als park_event gesloten en trip bezig sluit trip af en registreer park_event
b. nee
maak nieuw park event aan.

Checkout 
1. wanneer alle voertuigen tegelijkertijd weg
a. vermoeden van storing, print storing naar scherm en negeer nieuwe data en blijf oude data vasthouden van
2. wanneer enkele voertuigen weg.
sluit park_event af en start trip op.



