import { connect } from './casm.js';

// set up SVG canvas
const svg = d3.select("svg"),
    width = +svg.attr("width"),
    height = +svg.attr("height");

svg.attr("viewBox", [-width / 2, -height / 2, width, height]);


const simulation = d3.forceSimulation()
    .force("charge", d3.forceManyBody())
    .force("link", d3.forceLink().id(d => d.id))
    .force("x", d3.forceX())
    .force("y", d3.forceY())
    .on("tick", ticked);


let link = svg.append("g")
    .attr("stroke", "#999")
    .attr("stroke-opacity", 0.6)
    .selectAll("line");

let node = svg.append("g")
    .attr("stroke", "#fff")
    .attr("stroke-width", 1.5)
    .selectAll("circle");

window.addEventListener("DOMContentLoaded", () => {
    connect({
        url: eventurl(),
        onstep: R.compose(simupdate, graphupdate, graphcopy),
    });
});

// Make a shallow copy to protect against mutation, while
// recycling old nodes to preserve position and velocity.
const graphcopy = graph => {
    const old = new Map(node.data().map(d => [d.id, d]));
    graph.nodes = graph.nodes.map(d => Object.assign(old.get(d.id) || {}, d));
    graph.links = graph.links.map(d => Object.assign({}, d));

    return graph;
}


const graphupdate = graph => {
    node = node
        .data(graph.nodes, d => d.id)
        .join(enter => enter.append("circle")
            .attr("r", 5)
            .call(drag(simulation))
            .call(node => node.append("title").text(d => d.id)));

    link = link
        .data(graph.links, d => [d.source, d.target])
        .join("line");


    return graph;
}

const simupdate = graph => {
    simulation.nodes(graph.nodes);
    simulation.force("link").links(graph.links);
    simulation.alpha(1).restart().tick();
    ticked(); // render now!
}


function ticked() {
    node.attr("cx", d => d.x)
        .attr("cy", d => d.y);

    link.attr("x1", d => d.source.x)
        .attr("y1", d => d.source.y)
        .attr("x2", d => d.target.x)
        .attr("y2", d => d.target.y);
}


function drag(simulation) {
    function dragstarted(event, d) {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
    }

    function dragged(event, d) {
        d.fx = event.x;
        d.fy = event.y;
    }

    function dragended(event, d) {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
    }

    return d3.drag()
        .on("start", dragstarted)
        .on("drag", dragged)
        .on("end", dragended);
}

function eventurl() {
    let u = new URL(location);
    u.protocol = "ws:";
    u.pathname = "/events";
    return u;
}