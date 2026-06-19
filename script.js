document.getElementById("totalEvents")
.innerText = events.length;
container.innerHTML += `
<div class="event-card">

    <div class="event-image">
        💍
    </div>

    <div class="event-content">

        <h2>
            ${event.bride_name}
            &
            ${event.groom_name}
        </h2>

        <p>📅 ${event.event_date}</p>

        <p>📍 ${event.venue}</p>

        <a href="event.html?id=${event.id}">
            <button>View Invitation</button>
        </a>

    </div>

</div>
`;